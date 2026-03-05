const { createApp } = Vue

createApp({

  data() {
    return {
      socket: null,

      inboundMessages: [],
      numbers: [],
    }
  },

  mounted() {

    // load existing records first
    this.fetchInitialData()

    // then start realtime updates
    this.connectWebSocket()

  },

  methods: {

    async fetchInitialData() {

      const res = await fetch("http://localhost:8080/initial-data")

      const data = await res.json()

      this.inboundMessages = data.inbound_messages || []
      this.numbers = data.numbers || []
    },

    connectWebSocket() {

      console.log("Connecting WebSocket...")

      this.socket = new WebSocket("ws://localhost:8080/ws")

      this.socket.onopen = () => {
        console.log("WebSocket connected")
      }

      this.socket.onmessage = (event) => {

        const msg = JSON.parse(event.data)

        if (msg.collection === "inbound_messages") {
          this.inboundMessages.unshift(msg.data)
        }

        if (msg.collection === "numbers") {
          this.numbers.unshift(msg.data)
        }

      }

      this.socket.onerror = (error) => {
        console.error("WebSocket error:", error)
      }

      this.socket.onclose = () => {

        console.log("WebSocket disconnected. Reconnecting...")

        setTimeout(() => {
          this.connectWebSocket()
        }, 3000)

      }

    }

  }

}).mount("#app")