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
const showDeleteConfirm = ref(false)
const numberToDelete = ref('')
const isDeleting = ref(false)

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
    let phone = newNumber.value.replace(/\s+/g, '').trim()
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

const confirmDelete = (phone) => {
    numberToDelete.value = phone
    showDeleteConfirm.value = true
}

const cancelDelete = () => {
    showDeleteConfirm.value = false
    numberToDelete.value = ''
}

const removeFromWhitelist = async () => {
    if (!numberToDelete.value) return
    
    isDeleting.value = true
    const phone = numberToDelete.value
    const token = await getIdTokenSafe()
    
    try {
        const res = await fetch(`http://localhost:8080/whitelist?phone=${encodeURIComponent(phone)}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${token}`
            }
        })
        if (res.ok) {
            whitelist.value = whitelist.value.filter(item => item.phone !== phone)
            showDeleteConfirm.value = false
        } else {
            const data = await res.text()
            alert(data || 'Failed to remove number')
        }
    } catch (err) {
        console.error("Error removing from whitelist:", err)
        alert('Network error. Please try again.')
    } finally {
        isDeleting.value = false
        numberToDelete.value = ''
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
        <router-link to="/seeded-numbers" class="nav-item"><i class="fas fa-phone"></i> Seeded Numbers</router-link>
        <a href="#" class="nav-item"><i class="fas fa-broadcast-tower"></i> Monitoring</a>
        <a href="#" class="nav-item"><i class="fas fa-chart-line"></i> Analytics</a>
        <router-link to="/whitelist" class="nav-item active"><i class="fas fa-shield-alt"></i> Whitelist</router-link>
        <a href="#" class="nav-item"><i class="fas fa-cog"></i> Settings</a>
        <a href="#" @click.prevent="logout" class="nav-item" style="margin-top: auto; color: #ef4444;"><i class="fas fa-sign-out-alt"></i> Sign Out</a>
      </nav>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <header class="header-section">
        <div class="header-info">
            <h1>Whitelist Management <span class="shield-icon">🛡️</span></h1>
            <p>Restrict interactions to these authorized phone numbers only.</p>
        </div>
        
        <!-- Quick Add Section -->
        <div class="quick-add-card">
            <div class="quick-add-container">
                <div class="input-wrapper">
                    <TelInput 
                        v-model="newNumber" 
                        placeholder="+1 234 567 890" 
                        class="custom-tel-input"
                        @validate="onValidate"
                        @keyup.enter="isPhoneValid && addToWhitelist()"
                    />
                    <transition name="fade">
                      <span v-if="errorMessage" class="error-msg">
                          {{ errorMessage }}
                      </span>
                    </transition>
                </div>
                <button 
                    @click="addToWhitelist"
                    :disabled="!isPhoneValid"
                    class="add-btn"
                    :class="{ 'btn-enabled': isPhoneValid }"
                >
                    <i class="fas fa-plus"></i>
                    <span>Add Number</span>
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
                   <button 
                     @click="confirmDelete(item.phone)"
                     style="background: none; border: none; color: #94a3b8; cursor: pointer; padding: 0.5rem; border-radius: 6px; transition: all 0.2s;" 
                     onmouseover="this.style.color='#ef4444'; this.style.background='rgba(239, 68, 68, 0.1)'" 
                     onmouseout="this.style.color='#94a3b8'; this.style.background='none'"
                    >
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

    <!-- Custom Delete Modal -->
    <transition name="modal-fade">
      <div v-if="showDeleteConfirm" class="modal-overlay" @click.self="cancelDelete">
        <div class="modal-content">
          <div class="modal-header">
            <div class="modal-icon-container">
              <i class="fas fa-exclamation-triangle"></i>
            </div>
            <h3>Confirm Removal</h3>
          </div>
          <div class="modal-body">
            <p>Are you sure you want to remove <span class="highlight-number">{{ numberToDelete }}</span> from your whitelist? This action cannot be undone.</p>
          </div>
          <div class="modal-footer">
            <button class="modal-btn-cancel" @click="cancelDelete" :disabled="isDeleting">Cancel</button>
            <button class="modal-btn-delete" @click="removeFromWhitelist" :disabled="isDeleting">
              <span v-if="!isDeleting">Remove Number</span>
              <span v-else><i class="fas fa-spinner fa-spin"></i> Removing...</span>
            </button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.header-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2.5rem;
  gap: 2rem;
}

.header-info h1 {
  font-size: 1.85rem;
  font-weight: 700;
  margin: 0 0 0.5rem 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.shield-icon {
  font-size: 1.5rem;
  filter: drop-shadow(0 0 8px rgba(59, 130, 246, 0.3));
}

.header-info p {
  color: var(--text-secondary);
  font-size: 0.95rem;
  margin: 0;
}

.quick-add-card {
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(12px);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  padding: 1rem;
  min-width: 420px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
}

.quick-add-container {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.input-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.error-msg {
  color: #ef4444;
  font-size: 0.75rem;
  font-weight: 500;
  padding-left: 4px;
}

.add-btn {
  height: 42px;
  padding: 0 1.25rem;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
  background: #1e293b;
  color: #64748b;
  cursor: not-allowed;
  font-weight: 600;
  font-size: 0.85rem;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  gap: 0.5rem;
  white-space: nowrap;
}

.add-btn i {
  font-size: 0.8rem;
}

.add-btn.btn-enabled {
  background: #3b82f6;
  color: white;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.25);
  border: 1px solid var(--border-color);
}

.add-btn.btn-enabled:hover {
  background: #2563eb;
  transform: translateY(-1px);
  box-shadow: 0 6px 15px rgba(59, 130, 246, 0.35);
}

.add-btn.btn-enabled:active {
  transform: translateY(0);
}

/* Transitions */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}

/* Deep selection for TelInput styling to match theme */
:deep(.custom-tel-input .vue-tel-input) {
  border: 1px solid var(--border-color) !important;
  background-color: rgba(0, 0, 0, 0.2) !important;
  border-radius: 8px !important;
  height: 42px !important;
  transition: all 0.2s !important;
}

:deep(.custom-tel-input .vue-tel-input:focus-within) {
  border-color: var(--accent-blue) !important;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2) !important;
  background-color: rgba(0, 0, 0, 0.3) !important;
}

:deep(.custom-tel-input .vti__input) {
  background-color: transparent !important;
  color: white !important;
  font-size: 0.9rem !important;
}

:deep(.custom-tel-input .vti__dropdown) {
  background-color: rgba(255, 255, 255, 0.03) !important;
  border-right: 1px solid var(--border-color) !important;
  border-radius: 8px 0 0 8px !important;
}

:deep(.custom-tel-input .vti__dropdown:hover) {
  background-color: rgba(255, 255, 255, 0.08) !important;
}

:deep(.custom-tel-input .vti__selection .vti__country-code) {
  color: #94a3b8 !important;
}

:deep(.custom-tel-input .vti__dropdown-list) {
  background-color: #0f172a !important;
  border: 1px solid rgba(255, 255, 255, 0.1) !important;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.5) !important;
}

:deep(.custom-tel-input .vti__dropdown-item) {
  color: #f8fafc !important;
}

:deep(.custom-tel-input .vti__dropdown-item.highlighted) {
  background-color: #1e293b !important;
}

@media (max-width: 1024px) {
  .header-section {
    flex-direction: column;
    align-items: flex-start;
  }
  .quick-add-card {
    width: 100%;
    min-width: unset;
  }
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: #0f172a;
  border: 1px solid var(--border-color);
  border-radius: 16px;
  width: 100%;
  max-width: 400px;
  padding: 1.5rem;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
  animation: modal-slide-up 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes modal-slide-up {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

.modal-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.25rem;
  text-align: center;
}

.modal-icon-container {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.25rem;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: #f8fafc;
}

.modal-body p {
  color: var(--text-secondary);
  line-height: 1.6;
  text-align: center;
  font-size: 0.95rem;
  margin: 0;
}

.highlight-number {
  color: #f8fafc;
  font-weight: 600;
  font-family: 'JetBrains Mono', monospace;
}

.modal-footer {
  display: flex;
  gap: 0.75rem;
  margin-top: 1.75rem;
}

.modal-footer button {
  flex: 1;
  height: 44px;
  border-radius: 10px;
  font-weight: 600;
  font-size: 0.9rem;
  transition: all 0.2s;
  cursor: pointer;
}

.modal-btn-cancel {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
}

.modal-btn-cancel:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #f8fafc;
}

.modal-btn-delete {
  background: #ef4444;
  border: none;
  color: white;
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.2);
}

.modal-btn-delete:hover:not(:disabled) {
  background: #dc2626;
  transform: translateY(-1px);
}

.modal-btn-delete:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

/* Modal Transitions */
.modal-fade-enter-active, .modal-fade-leave-active {
  transition: opacity 0.3s ease;
}
.modal-fade-enter-from, .modal-fade-leave-to {
  opacity: 0;
}
</style>
