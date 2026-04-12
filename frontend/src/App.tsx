import { useState, useEffect, useRef, useCallback } from 'react'
import './App.css'

interface Message {
  id?: number
  user_id?: number
  username: string
  content: string
  created_at: string
}

interface UserInfo {
  user_id: number
  username: string
  chat_remaining: number
}

type Page = 'LOADING' | 'LOGIN' | 'REGISTER' | 'CHAT' | 'RATE_LIMITED'

function getAvatarColor(name: string): string {
  const colors = [
    '#7c5cfc', '#6366f1', '#8b5cf6', '#a855f7',
    '#ec4899', '#f43f5e', '#ef4444', '#f97316',
    '#eab308', '#22c55e', '#14b8a6', '#06b6d4',
    '#3b82f6', '#2563eb',
  ]
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }
  return colors[Math.abs(hash) % colors.length]
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function getWsUrl(): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/ws`
}

function getApiUrl(path: string): string {
  return `/api/${path}`
}

function App() {
  const [page, setPage] = useState<Page>('LOADING')
  const [user, setUser] = useState<UserInfo | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [inputValue, setInputValue] = useState('')
  const [isConnected, setIsConnected] = useState(false)
  const [chatRemaining, setChatRemaining] = useState(100)
  const [rateLimitMsg, setRateLimitMsg] = useState('')
  const [toast, setToast] = useState('')
  const [authError, setAuthError] = useState('')

  // Auth form states
  const [authUsername, setAuthUsername] = useState('')
  const [authPassword, setAuthPassword] = useState('')
  const [authConfirm, setAuthConfirm] = useState('')
  const [authLoading, setAuthLoading] = useState(false)

  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const showToast = useCallback((msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(''), 4000)
  }, [])

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, scrollToBottom])

  // Check session on mount
  useEffect(() => {
    checkSession()
  }, [])

  async function checkSession() {
    try {
      const res = await fetch(getApiUrl('me'), { credentials: 'same-origin' })
      if (res.ok) {
        const data: UserInfo = await res.json()
        setUser(data)
        setChatRemaining(data.chat_remaining)
        setPage('CHAT')
      } else {
        setPage('LOGIN')
      }
    } catch {
      setPage('LOGIN')
    }
  }

  const connectWebSocket = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const ws = new WebSocket(getWsUrl())

    ws.onopen = () => {
      setIsConnected(true)
    }

    ws.onmessage = (event) => {
      const payload = JSON.parse(event.data)

      if (payload.type === 'rate_limit') {
        setRateLimitMsg(payload.error || 'Batas chat harian tercapai')
        setChatRemaining(0)
        setPage('RATE_LIMITED')
        ws.close()
        return
      }

      if (payload.type === 'message' && payload.data) {
        setMessages(prev => [...prev, payload.data])
        setChatRemaining(prev => Math.max(0, prev - (payload.data.username === user?.username ? 1 : 0)))
      }
    }

    ws.onclose = () => {
      setIsConnected(false)
      // Only reconnect if still on chat page
      if (page === 'CHAT') {
        reconnectTimeoutRef.current = setTimeout(connectWebSocket, 3000)
      }
    }

    ws.onerror = () => {
      ws.close()
    }

    wsRef.current = ws
  }, [user, page])

  const loadMessages = useCallback(async () => {
    try {
      const res = await fetch(getApiUrl('messages'), { credentials: 'same-origin' })
      if (res.ok) {
        const data: Message[] = await res.json()
        setMessages(data || [])
      } else if (res.status === 401) {
        setPage('LOGIN')
      }
    } catch (err) {
      console.error('Failed to load messages:', err)
    }
  }, [])

  useEffect(() => {
    if (page !== 'CHAT' || !user) return

    loadMessages()
    connectWebSocket()

    return () => {
      wsRef.current?.close()
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [page, user, loadMessages, connectWebSocket])

  // ===== AUTH HANDLERS =====

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    setAuthError('')
    setAuthLoading(true)

    try {
      const res = await fetch(getApiUrl('login'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({ username: authUsername, password: authPassword }),
      })

      const data = await res.json()

      if (res.ok) {
        setUser(data)
        setChatRemaining(data.chat_remaining)
        setAuthUsername('')
        setAuthPassword('')
        setPage('CHAT')
      } else if (res.status === 429) {
        setAuthError(data.error || 'Terlalu banyak percobaan')
      } else {
        setAuthError(data.error || 'Login gagal')
      }
    } catch {
      setAuthError('Koneksi gagal. Coba lagi.')
    } finally {
      setAuthLoading(false)
    }
  }

  async function handleRegister(e: React.FormEvent) {
    e.preventDefault()
    setAuthError('')

    if (authPassword !== authConfirm) {
      setAuthError('Password tidak cocok')
      return
    }
    if (authPassword.length < 4) {
      setAuthError('Password minimal 4 karakter')
      return
    }
    if (authUsername.length < 2) {
      setAuthError('Username minimal 2 karakter')
      return
    }

    setAuthLoading(true)

    try {
      const res = await fetch(getApiUrl('register'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({ username: authUsername, password: authPassword }),
      })

      const data = await res.json()

      if (res.ok) {
        setUser(data)
        setChatRemaining(data.chat_remaining)
        setAuthUsername('')
        setAuthPassword('')
        setAuthConfirm('')
        showToast('Akun berhasil dibuat! 🎉')
        setPage('CHAT')
      } else if (res.status === 429) {
        setAuthError(data.error || 'Batas registrasi tercapai')
      } else if (res.status === 409) {
        setAuthError(data.error || 'Username sudah dipakai')
      } else {
        setAuthError(data.error || 'Registrasi gagal')
      }
    } catch {
      setAuthError('Koneksi gagal. Coba lagi.')
    } finally {
      setAuthLoading(false)
    }
  }

  async function handleLogout() {
    try {
      await fetch(getApiUrl('logout'), {
        method: 'POST',
        credentials: 'same-origin',
      })
    } catch { /* ignore */ }
    wsRef.current?.close()
    setUser(null)
    setMessages([])
    setPage('LOGIN')
  }

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault()
    const content = inputValue.trim()
    if (!content || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return

    if (chatRemaining <= 0) {
      setRateLimitMsg('Batas chat harian kamu tercapai (100/hari)')
      setPage('RATE_LIMITED')
      return
    }

    wsRef.current.send(JSON.stringify({ content }))
    setInputValue('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend(e)
    }
  }

  // ===== LOADING SCREEN =====
  if (page === 'LOADING') {
    return (
      <div className="loading-screen">
        <div className="loading-spinner" />
        <p>Memuat...</p>
      </div>
    )
  }

  // ===== LOGIN SCREEN =====
  if (page === 'LOGIN') {
    return (
      <div className="login-screen">
        <div className="login-card">
          <div className="login-logo">💬</div>
          <h1>Valk Chat</h1>
          <p>Masuk ke akun kamu</p>
          <form className="login-form" onSubmit={handleLogin}>
            <input
              id="login-username"
              className="login-input"
              type="text"
              placeholder="Username"
              value={authUsername}
              onChange={(e) => setAuthUsername(e.target.value)}
              autoFocus
              maxLength={20}
              autoComplete="username"
            />
            <input
              id="login-password"
              className="login-input"
              type="password"
              placeholder="Password"
              value={authPassword}
              onChange={(e) => setAuthPassword(e.target.value)}
              autoComplete="current-password"
            />
            {authError && <div className="auth-error">{authError}</div>}
            <button
              id="login-btn"
              className="login-btn"
              type="submit"
              disabled={authLoading || !authUsername.trim() || !authPassword}
            >
              {authLoading ? 'Memproses...' : 'Masuk'}
            </button>
          </form>
          <div className="auth-switch">
            Belum punya akun?{' '}
            <button
              className="auth-switch-btn"
              onClick={() => {
                setPage('REGISTER')
                setAuthError('')
              }}
            >
              Daftar
            </button>
          </div>
        </div>
      </div>
    )
  }

  // ===== REGISTER SCREEN =====
  if (page === 'REGISTER') {
    return (
      <div className="login-screen">
        <div className="login-card">
          <div className="login-logo">✨</div>
          <h1>Daftar Akun</h1>
          <p>Buat akun baru untuk mulai chat</p>
          <form className="login-form" onSubmit={handleRegister}>
            <input
              id="register-username"
              className="login-input"
              type="text"
              placeholder="Username (min. 2 karakter)"
              value={authUsername}
              onChange={(e) => setAuthUsername(e.target.value)}
              autoFocus
              maxLength={20}
              autoComplete="username"
            />
            <input
              id="register-password"
              className="login-input"
              type="password"
              placeholder="Password (min. 4 karakter)"
              value={authPassword}
              onChange={(e) => setAuthPassword(e.target.value)}
              autoComplete="new-password"
            />
            <input
              id="register-confirm"
              className="login-input"
              type="password"
              placeholder="Konfirmasi password"
              value={authConfirm}
              onChange={(e) => setAuthConfirm(e.target.value)}
              autoComplete="new-password"
            />
            {authError && <div className="auth-error">{authError}</div>}
            <button
              id="register-btn"
              className="login-btn"
              type="submit"
              disabled={authLoading || !authUsername.trim() || !authPassword || !authConfirm}
            >
              {authLoading ? 'Memproses...' : 'Daftar'}
            </button>
          </form>
          <div className="auth-switch">
            Sudah punya akun?{' '}
            <button
              className="auth-switch-btn"
              onClick={() => {
                setPage('LOGIN')
                setAuthError('')
              }}
            >
              Masuk
            </button>
          </div>
        </div>
      </div>
    )
  }

  // ===== RATE LIMITED SCREEN =====
  if (page === 'RATE_LIMITED') {
    return (
      <div className="rate-limit-screen">
        <div className="rate-limit-card">
          <div className="rate-limit-icon">⚠️</div>
          <h1>Batas Tercapai</h1>
          <p className="rate-limit-msg">{rateLimitMsg || 'Batas chat harian kamu tercapai (100/hari)'}</p>
          <div className="rate-limit-counter">
            <span className="counter-num">0</span>
            <span className="counter-label">pesan tersisa hari ini</span>
          </div>
          <p className="rate-limit-sub">Kuota chat akan direset besok. Terima kasih sudah menggunakan Valk Chat!</p>
          <button className="logout-btn" onClick={handleLogout}>
            Logout
          </button>
        </div>
      </div>
    )
  }

  // ===== CHAT SCREEN =====
  return (
    <div className="chat-container">
      {/* Toast */}
      {toast && <div className="toast">{toast}</div>}

      {/* Header */}
      <header className="chat-header">
        <div className="chat-header-left">
          <div className="chat-header-logo">💬</div>
          <div className="chat-header-info">
            <h2>Valk Chat</h2>
            <span>Real-time messaging</span>
          </div>
        </div>
        <div className="chat-header-right">
          <div className="chat-quota" title={`${chatRemaining} pesan tersisa hari ini`}>
            <span className="quota-icon">💬</span>
            <span className={`quota-count ${chatRemaining <= 10 ? 'low' : ''}`}>
              {chatRemaining}
            </span>
          </div>
          <div className="connection-status">
            <div className={`status-dot ${isConnected ? 'connected' : ''}`} />
            {isConnected ? 'Online' : 'Connecting...'}
          </div>
          <div className="user-badge">
            <div
              className="user-avatar-small"
              style={{ background: getAvatarColor(user?.username || '') }}
            >
              {user?.username?.[0]?.toUpperCase() || '?'}
            </div>
            {user?.username}
          </div>
          <button className="logout-header-btn" onClick={handleLogout} title="Logout">
            ⏻
          </button>
        </div>
      </header>

      {/* Messages */}
      <div className="messages-area" id="messages-area">
        {messages.length === 0 ? (
          <div className="messages-empty">
            <div className="messages-empty-icon">💬</div>
            <p>Belum ada pesan. Mulai percakapan!</p>
          </div>
        ) : (
          messages.map((msg, idx) => {
            const isOwn = msg.username === user?.username
            const prevMsg = idx > 0 ? messages[idx - 1] : null
            const isContinuation = prevMsg?.username === msg.username
            const showHeader = !isContinuation

            return (
              <div
                key={msg.id || `msg-${idx}`}
                className={`message ${isOwn ? 'own' : 'other'}`}
              >
                {showHeader && (
                  <div className="message-header">
                    {!isOwn && (
                      <div
                        className="message-avatar"
                        style={{ background: getAvatarColor(msg.username) }}
                      >
                        {msg.username[0].toUpperCase()}
                      </div>
                    )}
                    <span className="message-username">
                      {isOwn ? 'Kamu' : msg.username}
                    </span>
                    <span className="message-time">
                      {formatTime(msg.created_at)}
                    </span>
                  </div>
                )}
                <div className={`message-bubble ${isContinuation ? 'message-continuation' : ''}`}>
                  {msg.content}
                </div>
              </div>
            )
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="message-input-area">
        <form className="message-input-wrapper" onSubmit={handleSend}>
          <textarea
            id="message-input"
            className="message-input"
            placeholder="Ketik pesan..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            rows={1}
            autoComplete="off"
          />
          <button
            id="send-btn"
            className="send-btn"
            type="submit"
            disabled={!inputValue.trim() || !isConnected || chatRemaining <= 0}
            aria-label="Kirim pesan"
          >
            ➤
          </button>
        </form>
      </div>
    </div>
  )
}

export default App
