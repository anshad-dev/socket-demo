<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { auth } from '../firebase'
import { signOut, onAuthStateChanged } from 'firebase/auth'

const router = useRouter()
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
    const res = await fetch("http://localhost:8080/initial-data", {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
    const data = await res.json()
    numbers.value = data.numbers || []
  } catch (err) {
    console.error("Error fetching numbers:", err)
  }
}

const connectWebSocket = async (token) => {
  if (!isValidToken(token)) return
  try { socket.value?.close?.() } catch (_) {}
  socket.value = new WebSocket(`ws://localhost:8080/ws?token=${token}`)

  socket.value.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    if (msg.collection === "numbers") {
      numbers.value.unshift(msg.data)
    }
  }

  socket.value.onclose = () => {
    setTimeout(async () => {
      const newToken = await getIdTokenSafe(true)
      if (isValidToken(newToken)) connectWebSocket(newToken)
    }, 3000)
  }
}

const logout = async () => {
  await signOut(auth)
  router.push('/login')
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
  const user = await getCurrentUser();
  if (user) {
    const token = await getIdTokenSafe(true)
    if (token) {
        await fetchInitialData(token);
        connectWebSocket(token);
    }
  } else {
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
        <router-link to="/" class="nav-item"><i class="fas fa-th-large"></i> Dashboard</router-link>
        <router-link to="/seeded-numbers" class="nav-item active"><i class="fas fa-phone"></i> Seeded Numbers</router-link>
        <a href="#" class="nav-item"><i class="fas fa-broadcast-tower"></i> Monitoring</a>
        <router-link to="/analytics" class="nav-item"><i class="fas fa-chart-line"></i> Analytics</router-link>
        <router-link to="/whitelist" class="nav-item"><i class="fas fa-shield-alt"></i> Whitelist</router-link>
        <a href="#" class="nav-item"><i class="fas fa-cog"></i> Settings</a>
        <a href="#" @click.prevent="logout" class="nav-item" style="margin-top: auto; color: #ef4444;"><i class="fas fa-sign-out-alt"></i> Sign Out</a>
      </nav>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <header class="header">
        <h1>Seeded Numbers 📱</h1>
        <p>Manage and monitor the virtual phone numbers assigned to your account.</p>
      </header>

      <div class="table-container">
        <div class="table-header">
          <h3>Purchased Numbers</h3>
          <div style="font-size: 0.85rem; color: var(--text-secondary); background: rgba(255,255,255,0.05); padding: 0.4rem 0.8rem; border-radius: 20px;">
            {{ numbers.length }} Total Numbers
          </div>
        </div>
        
        <div style="overflow-x: auto;">
          <table style="width: 100%;">
            <thead>
              <tr>
                <th>Phone Number</th>
                <th>Purchased At</th>
                <th style="text-align: right;">Provisioning Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="num in numbers" :key="num._id" onmouseover="this.style.background='rgba(255,255,255,0.02)'" onmouseout="this.style.background='transparent'">
                <td style="font-weight: 600; color: #f8fafc; font-family: 'JetBrains Mono', monospace;">
                  {{ num.phone }}
                </td>
                <td style="color: var(--text-secondary); font-size: 0.9rem;">
                  {{ formatDate(num.created_at) }}
                </td>
                <td style="text-align: right;">
                  <span style="color: #10b981; font-weight: 500;"><i class="fas fa-check-circle"></i> Active & Provisioned</span>
                </td>
              </tr>
              <tr v-if="numbers.length === 0">
                <td colspan="3" style="text-align: center; color: var(--text-secondary); padding: 4rem;">
                  <div style="display: flex; flex-direction: column; align-items: center; gap: 1rem;">
                    <i class="fas fa-phone-slash" style="font-size: 2.5rem; opacity: 0.2;"></i>
                    <span>No numbers purchased yet.</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
/* No specific scoped styles needed for now */
</style>
