<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { auth } from '../firebase'
import { signOut, onAuthStateChanged } from 'firebase/auth'
import TelInput from '../components/TelInput.vue'

const router = useRouter()
const whitelist = ref([])
const newNumber = ref('')
const socket = ref(null)
const isPhoneValid = ref(false)

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

const fetchWhitelist = async (token) => {
  try {
    const res = await fetch("http://localhost:8080/whitelist", {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })
    const data = await res.json()
    whitelist.value = data || []
  } catch (err) {
    console.error("Error fetching whitelist:", err)
  }
}

const errorMessage = ref('')


const onValidate = (isValid) => {
    isPhoneValid.value = isValid
    if (!isValid && newNumber.value) {
        errorMessage.value = 'Invalid phone number format'
    } else {
        errorMessage.value = ''
    }
}

const addToWhitelist = async () => {
    errorMessage.value = ''
    let phone = newNumber.value.trim()
    if (!phone) return

    if (!isPhoneValid.value) {
        errorMessage.value = 'Please enter a valid phone number'
        return
    }
    
    const token = await getIdTokenSafe()
    try {
        const res = await fetch("http://localhost:8080/whitelist", {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ phone })
        })
        if (res.ok) {
            newNumber.value = ''
        } else {
            const data = await res.text()
            errorMessage.value = data || 'Failed to add number'
        }
    } catch (err) {
        console.error("Error adding to whitelist:", err)
        errorMessage.value = 'Network error. Please try again.'
    }
}


const connectWebSocket = async (token) => {
  if (!isValidToken(token)) return
  try { socket.value?.close?.() } catch (_) {}
  socket.value = new WebSocket(`ws://localhost:8080/ws?token=${token}`)

  socket.value.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    if (msg.collection === "whitelist") {
      whitelist.value.unshift(msg.data)
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
        await fetchWhitelist(token);
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
        <a href="#" class="nav-item"><i class="fas fa-phone"></i> Seeded Numbers</a>
        <a href="#" class="nav-item"><i class="fas fa-broadcast-tower"></i> Monitoring</a>
        <a href="#" class="nav-item"><i class="fas fa-chart-line"></i> Analytics</a>
        <router-link to="/whitelist" class="nav-item active"><i class="fas fa-shield-alt"></i> Whitelist</router-link>
        <a href="#" class="nav-item"><i class="fas fa-cog"></i> Settings</a>
        <a href="#" @click.prevent="logout" class="nav-item" style="margin-top: auto; color: #ef4444;"><i class="fas fa-sign-out-alt"></i> Sign Out</a>
      </nav>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <header class="header" style="display: flex; justify-content: space-between; align-items: flex-end; margin-bottom: 2rem;">
        <div style="flex: 1;">
            <h1>Whitelist Management 🛡️</h1>
            <p>Numbers explicitly allowed to interact with your seeded numbers.</p>
        </div>
        
        <!-- Quick Add Section -->
        <div class="card" style="margin: 0; min-width: 400px; padding: 1.25rem; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.08);">
            <div style="display: flex; gap: 0.75rem;">
                <div style="flex: 1; display: flex; flex-direction: column; gap: 0.5rem;">
                    <TelInput 
                        v-model="newNumber" 
                        placeholder="+1 234 567 890" 
                        @validate="onValidate"
                        @keyup.enter="isPhoneValid && addToWhitelist()"
                    />
                    <span v-if="errorMessage" style="color: #ef4444; font-size: 0.75rem; margin-top: -0.25rem; font-weight: 500;">
                        {{ errorMessage }}
                    </span>
                </div>
                <button 
                    @click="addToWhitelist"
                    :disabled="!isPhoneValid"
                    :style="{
                        height: '42px', 
                        padding: '0 1.25rem', 
                        borderRadius: '8px', 
                        border: 'none', 
                        background: isPhoneValid ? '#3b82f6' : '#1e293b', 
                        color: isPhoneValid ? 'white' : '#64748b', 
                        cursor: isPhoneValid ? 'pointer' : 'not-allowed', 
                        fontWeight: '600', 
                        fontSize: '0.85rem', 
                        transition: 'all 0.2s'
                    }"
                >
                    Add Number
                </button>
            </div>
        </div>
      </header>

      <!-- Main Table Section - Highlighted -->
      <div class="table-container" style="margin-top: 0; border: 1px solid rgba(255,255,255,0.1); background: rgba(255,255,255,0.02); overflow: hidden;">
        <div class="table-header" style="padding: 1.5rem; background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.05); display: flex; justify-content: space-between; align-items: center;">
          <div style="display: flex; align-items: center; gap: 0.75rem;">
            <div style="width: 10px; height: 10px; border-radius: 50%; background: #10b981; box-shadow: 0 0 10px rgba(16, 185, 129, 0.4);"></div>
            <h3 style="margin: 0; font-size: 1.1rem; letter-spacing: 0.02em;">Verified Whitelist</h3>
          </div>
          <div style="font-size: 0.85rem; color: var(--text-secondary); background: rgba(255,255,255,0.05); padding: 0.4rem 0.8rem; border-radius: 20px;">
            {{ whitelist.length }} Authorized Numbers
          </div>
        </div>
        
        <div style="overflow-x: auto;">
          <table>
            <thead>
              <tr style="background: rgba(0,0,0,0.1);">
                <th style="padding: 1.25rem 1.5rem;">Phone Number</th>
                <th>Authorised On</th>
                <th>Security Status</th>
                <th style="text-align: right; padding-right: 1.5rem;">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in whitelist" :key="item._id" style="border-bottom: 1px solid rgba(255,255,255,0.03); transition: background 0.2s;" onmouseover="this.style.background='rgba(255,255,255,0.02)'" onmouseout="this.style.background='transparent'">
                <td style="padding: 1.25rem 1.5rem; font-weight: 600; color: #f8fafc; font-family: 'JetBrains Mono', monospace;">
                  {{ item.phone }}
                </td>
                <td style="color: var(--text-secondary); font-size: 0.9rem;">
                  {{ formatDate(item.created_at) }}
                </td>
                <td>
                  <span style="display: inline-flex; align-items: center; gap: 0.5rem; padding: 0.35rem 0.75rem; border-radius: 20px; background: rgba(16, 185, 129, 0.1); color: #10b981; font-size: 0.75rem; font-weight: 600; border: 1px solid rgba(16, 185, 129, 0.2);">
                    <i class="fas fa-check-shield" style="font-size: 0.7rem;"></i> Verified
                  </span>
                </td>
                <td style="text-align: right; padding-right: 1.5rem;">
                   <button style="background: none; border: none; color: #94a3b8; cursor: pointer; padding: 0.5rem; border-radius: 6px; transition: all 0.2s;" onmouseover="this.style.color='#ef4444'; this.style.background='rgba(239, 68, 68, 0.1)'" onmouseout="this.style.color='#94a3b8'; this.style.background='none'">
                     <i class="fas fa-trash"></i>
                   </button>
                </td>
              </tr>
              <tr v-if="whitelist.length === 0">
                <td colspan="4" style="text-align: center; color: var(--text-secondary); padding: 4rem;">
                  <div style="display: flex; flex-direction: column; align-items: center; gap: 1rem;">
                    <i class="fas fa-shield-alt" style="font-size: 2.5rem; opacity: 0.2;"></i>
                    <span>No whitelisted numbers yet. Start by adding one above.</span>
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
