/// <reference types="vite/client" />

// Type declarations for CSS imports
declare module '*.css'

// Type declarations for Wails runtime
declare global {
  interface Window {
    go?: any;
    runtime?: any;
  }
}

