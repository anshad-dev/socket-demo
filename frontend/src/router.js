import { createRouter, createWebHistory } from 'vue-router'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Whitelist from './views/Whitelist.vue'
import { auth } from './firebase'
import { onAuthStateChanged } from 'firebase/auth'

const routes = [
    {
        path: '/login',
        name: 'Login',
        component: Login,
        meta: { public: true }
    },
    {
        path: '/',
        name: 'Dashboard',
        component: Dashboard
    },
    {
        path: '/whitelist',
        name: 'Whitelist',
        component: Whitelist
    },
    {
        path: '/:pathMatch(.*)*',
        redirect: '/'
    }
]

const router = createRouter({
    history: createWebHistory(),
    routes
})

// Navigation Guard
router.beforeEach((to, from, next) => {
    const isPublic = to.meta.public

    // Wait for Firebase to check the user's initial state
    const checkAuth = () => {
        return new Promise((resolve) => {
            const unsubscribe = onAuthStateChanged(auth, (user) => {
                unsubscribe()
                resolve(user)
            })
        })
    }

    checkAuth().then(user => {
        if (!user && !isPublic) {
            next('/login')
        } else if (user && isPublic) {
            next('/')
        } else {
            next()
        }
    })
})

export default router
