<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { auth } from '../firebase'
import { signOut, onAuthStateChanged } from 'firebase/auth'

const router = useRouter()
const inboundMessages = ref([])
const numbers = ref([])
const socket = ref(null)

const getCurrentUser = () => {
  return new Promise((resolve, reject) => {
    const unsubscribe = onAuthStateChanged(auth, (user) => {
      unsubscribe()
      resolve(user)
    }, reject)
  })
}

const isValidToken = (token) => {
  return typeof token === 'string' && token.trim() !== '' && token !== 'undefined' && token !== 'null'
}

const getIdTokenSafe = async (forceRefresh = false) => {
  try {
    const user = auth.currentUser || await getCurrentUser()
    if (!user) return null
    const token = await user.getIdToken(forceRefresh)
    return isValidToken(token) ? token : null
  } catch (e) {
    console.warn('Failed to get Firebase ID token:', e)
    return null
  }
}

const fetchInitialData = async (token) => {
  try {
    if (!isValidToken(token)) {
      console.warn("fetchInitialData: No valid token.")
      return
    }

    const res = await fetch("http://localhost:8080/initial-data", {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
    const data = await res.json()
    inboundMessages.value = data.inbound_messages || []
    numbers.value = data.numbers || []
  } catch (err) {
    console.error("Error fetching initial data:", err)
  }
}

const connectWebSocket = async (token) => {
  console.log("Connecting WebSocket...")
  if (!isValidToken(token)) {
    console.warn("connectWebSocket: No valid token.")
    return
  }
  // Close any existing connection first (prevents duplicate sockets during fast remounts/hot reload)
  try { socket.value?.close?.() } catch (_) {}
  socket.value = new WebSocket(`ws://localhost:8080/ws?token=${token}`)

  socket.value.onopen = () => {
    console.log("WebSocket connected")
  }

  socket.value.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    if (msg.collection === "inbound_messages") {
      inboundMessages.value.unshift(msg.data)
    }
    if (msg.collection === "numbers") {
      numbers.value.unshift(msg.data)
    }
  }

  socket.value.onerror = (error) => {
    console.error("WebSocket error:", error)
  }

  socket.value.onclose = () => {
    console.log("WebSocket disconnected. Reconnecting in 3s...")
    setTimeout(async () => {
      const newToken = await getIdTokenSafe(true)
      if (isValidToken(newToken)) {
        connectWebSocket(newToken)
      } else {
        console.warn("Reconnection skipped: No valid session.")
      }
    }, 3000)
  }
}

const logout = async () => {
  await signOut(auth)
  router.push('/login')
}

const syncUser = async (token) => {
  try {
    if (!isValidToken(token)) {
      console.warn("SyncUser skipped: No active Firebase token.")
      return
    }
    const res = await fetch("http://localhost:8080/sync-user", {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
    console.log("SyncUser Response Status:", res.status)
  } catch (err) {
    console.error("Error syncing user:", err)
  }
}

const formatDate = (dateStr) => {
  if (!dateStr) return '—'
  try {
    const date = new Date(dateStr)
    return date.toLocaleString('en-US', { 
      month: 'short', 
      day: 'numeric', 
      hour: '2-digit', 
      minute: '2-digit',
      hour12: true 
    })
  } catch (e) {
    return dateStr
  }
}

onMounted(async () => {
  console.log("Dashboard: Waiting for authentication...");
  const user = await getCurrentUser();
  
  if (user) {
    const token = await getIdTokenSafe(true)
    console.log("Dashboard: Auth success. Token length:", token?.length)

    if (!isValidToken(token)) {
      console.error("Dashboard: Error - Token is invalid!")
      return;
    }

    await syncUser(token);
    await fetchInitialData(token);
    connectWebSocket(token);
  } else {
    console.warn("Dashboard: No user session found. Redirecting to login.");
    router.push('/login');
  }
});
</script>

<template>
  <div id="app" style="display: contents;">
    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="logo">
        <i class="fas fa-cube"></i>
        <span>SeedTrack</span>
      </div>
      
      <nav class="nav-links">
        <a href="#" class="nav-item active"><i class="fas fa-th-large"></i> Dashboard</a>
        <a href="#" class="nav-item"><i class="fas fa-phone"></i> Seeded Numbers</a>
        <a href="#" class="nav-item"><i class="fas fa-broadcast-tower"></i> Monitoring</a>
        <a href="#" class="nav-item"><i class="fas fa-chart-line"></i> Analytics</a>
        <a href="#" class="nav-item"><i class="fas fa-shield-alt"></i> Whitelist</a>
        <a href="#" class="nav-item"><i class="fas fa-cog"></i> Settings</a>
        <a href="#" @click.prevent="logout" class="nav-item" style="margin-top: auto; color: #ef4444;"><i class="fas fa-sign-out-alt"></i> Sign Out</a>
      </nav>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <header class="header">
        <h1>Welcome to SeedTrack Pro 👋</h1>
        <p>Your numbers are live and ready. We'll alert you when inbound activity is detected.</p>
      </header>

      <!-- Stats Grid -->
      <div class="dashboard-grid">
        <div class="card">
          <div class="card-title">Active Seeded Numbers</div>
          <div class="card-value">{{ numbers.length }} / 1000</div>
        </div>
        <div class="card">
          <div class="card-title">Inbound Events Today</div>
          <div class="card-value">{{ inboundMessages.length }}</div>
        </div>
        <div class="card">
          <div class="card-title">Unique Senders</div>
          <div class="card-value">—</div>
        </div>
        <div class="card">
          <div class="card-title">Repeat Senders</div>
          <div class="card-value">— %</div>
        </div>
      </div>

      <!-- Center Status Section -->
      <div class="status-container">
        <div class="visual-section">
          <div class="listening-icon" :class="{ 'active': inboundMessages.length > 0 }">
            <i class="fas fa-signal" style="color: white; font-size: 1.5rem;"></i>
          </div>
          
          <div v-if="inboundMessages.length > 0" class="latest-event fade-in">
            <div class="badge" style="margin-bottom: 0.5rem;">New Inbound Message</div>
            <h2 class="visual-title" style="margin-top: 0;">{{ inboundMessages[0].body }}</h2>
            <p class="visual-desc">
              From: {{ inboundMessages[0].from }} → To: {{ inboundMessages[0].to }}
            </p>
          </div>
          
          <div v-else>
            <h2 class="visual-title">Listening for Inbound Events...</h2>
            <p class="visual-desc">
              Seeded numbers have been successfully embedded. 
              When someone contacts them, inbound activity will appear here.
            </p>
          </div>
        </div>

        <div class="status-sidebar">
          <div class="status-card">
            <div class="status-header">
              <i class="fas fa-circle" style="font-size: 0.5rem; color: #10b981;"></i>
              <span>System Status</span>
            </div>
            <div class="status-detail">
              <span class="status-label">Monitoring Active</span>
            </div>
            <div class="status-detail">
              <span class="status-label">Provider:</span>
              <span>Twilio</span>
            </div>
            <div class="status-detail">
              <span class="status-label">Webhook:</span>
              <span style="color: #10b981;">Connected</span>
            </div>
          </div>
          
          <div class="status-card">
             <div class="status-header">
              <i class="fas fa-phone-alt" style="font-size: 0.8rem; color: #3b82f6;"></i>
              <span>Purchased Numbers</span>
            </div>
            <div style="max-height: 200px; overflow-y: auto; padding-right: 0.5rem;" class="custom-scrollbar">
                <div v-for="num in numbers" :key="num.phone" class="status-detail" style="padding: 0.5rem 0; border-bottom: 1px solid rgba(255,255,255,0.05);">
                    <div style="display: flex; flex-direction: column;">
                        <span style="font-weight: 500;">{{ num.phone }}</span>
                        <span style="font-size: 0.7rem; color: var(--text-secondary);">User ID: {{ num.userId }}</span>
                    </div>
                </div>
                <div v-if="numbers.length === 0" class="status-label" style="text-align: center; padding: 1rem;">
                    No numbers purchased.
                </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Inbound Events Table -->
      <div class="table-container">
        <div class="table-header">
          <h3>Recent Inbound Events</h3>
          <div style="font-size: 0.8rem; color: var(--text-secondary);">
            <i class="fas fa-sync-alt"></i> Live Updating
          </div>
        </div>
        <table>
          <thead>
            <tr>
              <th>From</th>
              <th>To</th>
              <th>Message</th>
              <th>Received At</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="msg in inboundMessages" :key="msg.id || msg.from + msg.timestamp">
              <td>{{ msg.from }}</td>
              <td>{{ msg.to }}</td>
              <td>{{ msg.body }}</td>
              <td>{{ formatDate(msg.received_at) }}</td>
              <td style="color: #10b981;">Success</td>
            </tr>
            <tr v-if="inboundMessages.length === 0">
              <td colspan="5" style="text-align: center; color: var(--text-secondary); padding: 3rem;">
                No inbound events detected yet.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>

<style scoped>
/* Scoped styles can go here if needed, but we're using global style.css */
</style>
