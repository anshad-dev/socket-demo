<template>
  <div class="login-page">
    <div class="login-card glass">
      <div class="logo">
        <i class="fas fa-cube"></i>
        <span>SeedTrack Pro</span>
      </div>
      <h2>Welcome Back</h2>
      <p>Secure authentication for elite marketers.</p>
      
      <div class="auth-methods">
        <button @click="loginWithGoogle" class="btn google-btn">
          <i class="fab fa-google"></i> Continue with Google
        </button>
      </div>
      
      <div class="divider">
        <span>or</span>
      </div>
      
      <form @submit.prevent="loginWithEmail" class="login-form">
        <div class="form-group">
          <label>Project ID / Email</label>
          <input type="email" v-model="email" placeholder="name@company.com" required>
        </div>
        <div class="form-group">
          <label>Access Key</label>
          <input type="password" v-model="password" placeholder="••••••••" required>
        </div>
        <button type="submit" class="btn submit-btn" :disabled="loading">
          {{ loading ? 'Authenticating...' : 'Sign In' }}
        </button>
        <p v-if="error" class="error-msg">{{ error }}</p>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { auth, googleProvider } from '../firebase'
import { signInWithPopup, signInWithEmailAndPassword } from 'firebase/auth'

const router = useRouter()
const email = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

const loginWithGoogle = async () => {
  error.value = ''
  loading.value = true
  try {
    await signInWithPopup(auth, googleProvider)
    router.push('/')
  } catch (err) {
    error.value = "Google authentication failed. Please try again."
    console.error(err)
  } finally {
    loading.value = false
  }
}

const loginWithEmail = async () => {
  error.value = ''
  loading.value = true
  try {
    await signInWithEmailAndPassword(auth, email.value, password.value)
    router.push('/')
  } catch (err) {
    error.value = "Invalid credentials or account non-existent."
    console.error(err)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  height: 100vh;
  width: 100vw;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #05070a;
  background: radial-gradient(circle at center, rgba(59, 130, 246, 0.1) 0%, transparent 70%);
}

.login-card {
  width: 100%;
  max-width: 400px;
  padding: 3rem;
  border-radius: 1.5rem;
  background: rgba(15, 20, 35, 0.7);
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.05);
  text-align: center;
}

.logo {
  font-size: 2rem;
  font-weight: 700;
  color: #3b82f6;
  margin-bottom: 2rem;
  justify-content: center;
}

h2 {
  font-size: 1.5rem;
  margin-bottom: 0.5rem;
  color: #f8fafc;
}

p {
  color: #94a3b8;
  margin-bottom: 2rem;
  font-size: 0.9rem;
}

.auth-methods {
  margin-bottom: 1.5rem;
}

.btn {
  width: 100%;
  padding: 0.75rem;
  border-radius: 0.75rem;
  border: none;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
}

.google-btn {
  background: white;
  color: #05070a;
}

.google-btn:hover {
  background: #f1f5f9;
}

.divider {
  display: flex;
  align-items: center;
  gap: 1rem;
  color: #475569;
  margin: 1.5rem 0;
}

.divider::before, .divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: rgba(255, 255, 255, 0.1);
}

.login-form {
  text-align: left;
}

.form-group {
  margin-bottom: 1.25rem;
}

.form-group label {
  display: block;
  font-size: 0.8rem;
  color: #94a3b8;
  margin-bottom: 0.5rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

input {
  width: 100%;
  padding: 0.75rem 1rem;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 0.5rem;
  color: white;
  font-size: 0.9rem;
}

input:focus {
  outline: none;
  border-color: #3b82f6;
  background: rgba(59, 130, 246, 0.05);
}

.submit-btn {
  background: #3b82f6;
  color: white;
  margin-top: 1rem;
}

.submit-btn:hover {
  background: #2563eb;
}

.error-msg {
  color: #ef4444;
  font-size: 0.8rem;
  margin-top: 1rem;
  text-align: center;
}
</style>
