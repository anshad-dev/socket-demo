<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { auth } from '../firebase'
import { signOut, onAuthStateChanged } from 'firebase/auth'
import Chart from 'chart.js/auto'

const router = useRouter()
const stats = ref({
    volume_chart: [],
    sender_stats: { total: 0, unique: 0, repeated: 0 },
    top_senders: [],
    top_numbers: [],
    available_numbers: []
})
const isLoading = ref(true)
const selectedDays = ref(7)
const selectedPhone = ref('')
let volumeChart = null

const getCurrentUser = () => {
  return new Promise((resolve, reject) => {
    const unsubscribe = onAuthStateChanged(auth, (user) => {
      unsubscribe()
      resolve(user)
    }, reject)
  })
}

const getIdTokenSafe = async (forceRefresh = false) => {
  try {
    const user = auth.currentUser || await getCurrentUser()
    if (!user) return null
    return await user.getIdToken(forceRefresh)
  } catch (e) {
    return null
  }
}

const fetchAnalytics = async () => {
  const token = await getIdTokenSafe()
  if (!token) return

  isLoading.value = true
  try {
    const url = new URL("http://localhost:8080/analytics")
    url.searchParams.append("days", selectedDays.value)
    if (selectedPhone.value) {
        url.searchParams.append("phone", selectedPhone.value)
    }

    const res = await fetch(url.toString(), {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    const data = await res.json()
    // Ensure nested objects exist to prevent template errors
    stats.value = {
        volume_chart: data.volume_chart || [],
        sender_stats: data.sender_stats || { total: 0, unique: 0, repeated: 0 },
        top_senders: data.top_senders || [],
        top_numbers: data.top_numbers || [],
        available_numbers: data.available_numbers || []
    }
    isLoading.value = false
    await nextTick()
    renderCharts()
  } catch (err) {
    console.error("Error fetching analytics:", err)
    isLoading.value = false
  }
}

const renderCharts = () => {
    // Small delay to ensure DOM is fully ready even after nextTick
    setTimeout(() => {
        const canvas = document.getElementById('volumeChart')
        if (!canvas) {
            console.warn("Chart canvas not found")
            return
        }

        const ctx = canvas.getContext('2d')
        if (!ctx) return

        if (volumeChart) volumeChart.destroy()

        const labels = stats.value.volume_chart?.map(d => d._id) || []
        const data = stats.value.volume_chart?.map(d => d.count) || []

        console.log("Rendering chart with labels:", labels)

        volumeChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Inbound Volume',
                    data: data,
                    borderColor: '#3b82f6',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    fill: true,
                    tension: 0.4,
                    borderWidth: 3,
                    pointBackgroundColor: '#3b82f6',
                    pointRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        grid: { color: 'rgba(255, 255, 255, 0.05)' },
                        ticks: { color: '#94a3b8' }
                    },
                    x: {
                        grid: { display: false },
                        ticks: { color: '#94a3b8' }
                    }
                }
            }
        })
    }, 100)
}

const logout = async () => {
  await signOut(auth)
  router.push('/login')
}

onMounted(() => {
  fetchAnalytics()
})

onUnmounted(() => {
    if (volumeChart) volumeChart.destroy()
})
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
        <router-link to="/analytics" class="nav-item active"><i class="fas fa-chart-line"></i> Analytics</router-link>
        <router-link to="/whitelist" class="nav-item"><i class="fas fa-shield-alt"></i> Whitelist</router-link>
        <a href="#" class="nav-item"><i class="fas fa-cog"></i> Settings</a>
        <a href="#" @click.prevent="logout" class="nav-item" style="margin-top: auto; color: #ef4444;"><i class="fas fa-sign-out-alt"></i> Sign Out</a>
      </nav>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <header class="header" style="display: flex; justify-content: space-between; align-items: flex-end; margin-bottom: 2.5rem;">
        <div>
            <h1>Analytics Insights 📊</h1>
            <p>Comprehensive breakdown of your inbound traffic and sender patterns.</p>
        </div>

        <div class="filters-container">
            <!-- Days Filter (Segmented) -->
            <div class="segment-filter">
                <button 
                    v-for="d in [1, 3, 5, 7]" 
                    :key="d" 
                    @click="selectedDays = d; fetchAnalytics()"
                    :class="{ active: selectedDays === d }"
                >
                    {{ d }}d
                </button>
            </div>

            <!-- Phone Filter -->
            <div class="select-wrapper">
                <i class="fas fa-phone"></i>
                <select v-model="selectedPhone" @change="fetchAnalytics">
                    <option value="">All Numbers</option>
                    <option v-for="num in stats.available_numbers" :key="num" :value="num">
                        {{ num }}
                    </option>
                </select>
            </div>
        </div>
      </header>

      <div v-if="isLoading" class="loading-state">
          <i class="fas fa-circle-notch fa-spin"></i>
          <span>Gathering data...</span>
      </div>

      <div v-else class="analytics-grid">
        <!-- Main Chart -->
        <div class="card chart-card">
          <div class="card-header">
            <h3>Inbound Volume</h3>
            <span class="trend positive"><i class="fas fa-arrow-up"></i> Last {{ selectedDays }} Days</span>
          </div>
          <div class="chart-container">
            <canvas id="volumeChart"></canvas>
          </div>
        </div>

        <!-- Sender Stats -->
        <div class="card stats-card">
          <h3>Sender Engagement</h3>
          <div class="donut-container">
            <div class="stat-item">
              <span class="label">Total Distinct Senders</span>
              <span class="value">{{ stats.sender_stats.total }}</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-item">
              <span class="label">Returning Senders (Multiple msgs)</span>
              <span class="value">{{ stats.sender_stats.repeated }}</span>
            </div>
            <div class="stat-divider"></div>
            <div class="stat-item">
              <span class="label">One-time Senders (Single msg)</span>
              <span class="value">{{ stats.sender_stats.unique }}</span>
            </div>
          </div>
        </div>

        <!-- Top Senders -->
        <div class="card list-card">
          <h3>Top Senders</h3>
          <div class="rank-list">
            <div v-for="(sender, index) in stats.top_senders" :key="sender._id" class="rank-item">
              <div class="rank-number">#{{ index + 1 }}</div>
              <div class="rank-info">
                <span class="rank-name">{{ sender._id }}</span>
                <span class="rank-count">{{ sender.count }} messages</span>
              </div>
            </div>
            <div v-if="stats.top_senders.length === 0" class="empty-state">No data available</div>
          </div>
        </div>

        <!-- Messages per Number -->
        <div class="card list-card">
          <h3>Traffic per Number</h3>
          <div class="rank-list">
            <template v-if="stats.top_numbers?.length">
              <div v-for="item in stats.top_numbers" :key="item._id" class="rank-item">
                <div class="rank-info">
                  <span class="rank-name">{{ item._id }}</span>
                  <div class="progress-bar">
                      <div class="progress-fill" :style="{ width: (stats.top_numbers[0]?.count ? (item.count / stats.top_numbers[0].count * 100) : 0) + '%' }"></div>
                  </div>
                </div>
                <span class="rank-count">{{ item.count }}</span>
              </div>
            </template>
            <div v-else class="empty-state">No data available for this selection</div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 400px;
    gap: 1rem;
    color: var(--text-secondary);
}

.loading-state i { font-size: 2rem; color: var(--accent-blue); }

.filters-container {
    display: flex;
    gap: 1.5rem;
    align-items: center;
}

.segment-filter {
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid var(--border-color);
    padding: 2px;
    border-radius: 10px;
    display: flex;
    gap: 2px;
}

.segment-filter button {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    padding: 0.5rem 1rem;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
}

.segment-filter button:hover {
    color: var(--text-primary);
}

.segment-filter button.active {
    background: var(--accent-blue);
    color: white;
}

.select-wrapper {
    position: relative;
    display: flex;
    align-items: center;
}

.select-wrapper i {
    position: absolute;
    left: 1rem;
    font-size: 0.8rem;
    color: var(--accent-blue);
    pointer-events: none;
}

.select-wrapper select {
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid var(--border-color);
    border-radius: 10px;
    color: var(--text-primary);
    padding: 0.6rem 2.5rem;
    font-size: 0.85rem;
    cursor: pointer;
    outline: none;
    appearance: none;
    min-width: 180px;
}

.select-wrapper::after {
    content: '\f078';
    font-family: 'Font Awesome 5 Free';
    font-weight: 900;
    position: absolute;
    right: 1rem;
    font-size: 0.7rem;
    color: var(--text-secondary);
    pointer-events: none;
}

.analytics-grid {
    display: grid;
    grid-template-columns: 2fr 1fr;
    gap: 1.5rem;
}

.chart-card { grid-column: span 1; padding: 1.5rem; }
.stats-card { padding: 1.5rem; display: flex; flex-direction: column; justify-content: center; }

.card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.5rem;
}

.trend {
    font-size: 0.75rem;
    padding: 0.25rem 0.6rem;
    border-radius: 20px;
    background: rgba(16, 185, 129, 0.1);
    color: #10b981;
}

.chart-container { height: 300px; position: relative; }

.donut-container {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    margin-top: 2rem;
}

.stat-item {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.stat-item .label { color: var(--text-secondary); font-size: 0.85rem; }
.stat-item .value { font-size: 2rem; font-weight: 700; color: #f8fafc; }

.stat-divider { height: 1px; background: rgba(255, 255, 255, 0.05); }

.list-card { padding: 1.5rem; }

.rank-list {
    margin-top: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
}

.rank-item {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem;
    background: rgba(255, 255, 255, 0.02);
    border-radius: 8px;
}

.rank-number {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--accent-blue);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 700;
}

.rank-info { flex: 1; display: flex; flex-direction: column; gap: 0.25rem; }
.rank-name { font-size: 0.9rem; font-weight: 600; color: #f8fafc; }
.rank-count { font-size: 0.8rem; color: var(--text-secondary); }

.progress-bar {
    height: 4px;
    background: rgba(255, 255, 255, 0.05);
    border-radius: 2px;
    overflow: hidden;
}

.progress-fill {
    height: 100%;
    background: var(--accent-blue);
    transition: width 0.5s ease;
}

.empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--text-secondary);
    font-size: 0.9rem;
}

@media (max-width: 1200px) {
    .analytics-grid { grid-template-columns: 1fr; }
}
</style>
