import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'

export type Theme = 'light' | 'dark'

interface ThemeContextType {
  theme: Theme
  setTheme: (t: Theme) => void
}

const ThemeContext = createContext<ThemeContextType>({
  theme: 'light',
  setTheme: () => {},
})

function applyTheme(theme: Theme) {
  if (theme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>('light')

  useEffect(() => {
    // 从后端加载偏好设置
    const isDesktop = typeof window !== 'undefined' && 'go' in window
    if (isDesktop) {
      // @ts-ignore
      void (window.go.desktop.App.GetUIPrefs() as Promise<{ language: string; theme: string }>).then((prefs) => {
        const t = prefs.theme === 'dark' ? 'dark' : 'light'
        setThemeState(t)
        applyTheme(t)
      }).catch(() => {})
    }
  }, [])

  const setTheme = (t: Theme) => {
    setThemeState(t)
    applyTheme(t)
  }

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  return useContext(ThemeContext)
}
