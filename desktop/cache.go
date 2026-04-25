package desktop

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"io"
	"sync"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// KubeconfigCache 是桌面应用专用的缓存，支持加密存储
type KubeconfigCache struct {
	mu         sync.RWMutex
	entries    map[string][]byte // 存储加密后的数据
	encryptKey []byte            // AES 加密密钥 (32 bytes for AES-256)
}

// NewKubeconfigCache 创建新的缓存实例，生成随机加密密钥
func NewKubeconfigCache() *KubeconfigCache {
	// 生成 32 字节的随机密钥用于 AES-256 加密
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		klog.Fatalf("Failed to generate encryption key: %v", err)
	}
	klog.Info("Generated session encryption key for kubeconfig cache")
	return &KubeconfigCache{
		entries:    make(map[string][]byte),
		encryptKey: key,
	}
}

// encrypt 使用 AES-GCM 加密数据
func (c *KubeconfigCache) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decrypt 使用 AES-GCM 解密数据
func (c *KubeconfigCache) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.encryptKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// Get 获取指定集群的 kubeconfig（自动解密）
func (c *KubeconfigCache) Get(clusterName string) (*rest.Config, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	encryptedData, ok := c.entries[clusterName]
	if !ok {
		return nil, false
	}

	// 解密数据
	decryptedData, err := c.decrypt(encryptedData)
	if err != nil {
		klog.Errorf("Failed to decrypt kubeconfig for cluster %s: %v", clusterName, err)
		return nil, false
	}

	// 反序列化 rest.Config
	var cfg rest.Config
	decoder := gob.NewDecoder(bytes.NewReader(decryptedData))
	if err := decoder.Decode(&cfg); err != nil {
		klog.Errorf("Failed to deserialize kubeconfig for cluster %s: %v", clusterName, err)
		return nil, false
	}

	return &cfg, true
}

// Set 设置指定集群的 kubeconfig（自动加密）
func (c *KubeconfigCache) Set(clusterName string, cfg *rest.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 序列化 rest.Config
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(cfg); err != nil {
		klog.Errorf("Failed to serialize kubeconfig for cluster %s: %v", clusterName, err)
		return
	}

	// 加密数据
	encryptedData, err := c.encrypt(buf.Bytes())
	if err != nil {
		klog.Errorf("Failed to encrypt kubeconfig for cluster %s: %v", clusterName, err)
		return
	}

	c.entries[clusterName] = encryptedData
	klog.V(2).Infof("Cached encrypted kubeconfig for cluster: %s (%d bytes encrypted)", clusterName, len(encryptedData))
}

// Has 检查是否已缓存指定集群
func (c *KubeconfigCache) Has(clusterName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.entries[clusterName]
	return ok
}

// Clear 清除所有缓存
func (c *KubeconfigCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 清除加密数据
	c.entries = make(map[string][]byte)
	klog.Info("Cache cleared (encrypted entries removed)")
}

// List 列出所有已缓存的集群名称
func (c *KubeconfigCache) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.entries))
	for name := range c.entries {
		names = append(names, name)
	}
	return names
}
