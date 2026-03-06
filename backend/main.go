package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/api/option"
)

type contextKey string

const (
	uidKey contextKey = "uid"
)

var (
	clients = make(map[*websocket.Conn]string)
	mutex   = sync.Mutex{}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	client *mongo.Client
	db     *mongo.Database

	authClient *auth.Client
)

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func main() {

	connectMongo()
	initFirebase()

	go watchMongoChanges()

	http.Handle("/ws", AuthMiddleware(http.HandlerFunc(handleWebSocket)))
	http.HandleFunc("/twilio/inbound", handleTwilioInbound)
	http.Handle("/initial-data", AuthMiddleware(http.HandlerFunc(handleInitialData)))
	http.Handle("/sync-user", AuthMiddleware(http.HandlerFunc(handleSyncUser)))
	http.Handle("/whitelist", AuthMiddleware(http.HandlerFunc(handleWhitelist)))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Server running on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func connectMongo() {

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	var err error
	client, err = mongo.Connect(context.TODO(),
		options.Client().ApplyURI(mongoURI))

	if err != nil {
		log.Fatal(err)
	}

	db = client.Database("socket_demo")

	log.Println("MongoDB connected")
}

func initFirebase() {
	opt := option.WithCredentialsFile("serviceAccountKey.json")
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	authClient, err = app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}
	log.Println("Firebase initialized")
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		idToken := r.Header.Get("Authorization")
		source := "header"
		if idToken == "" {
			idToken = r.URL.Query().Get("token")
			source = "query"
		}

		if idToken == "" {
			// WebSocket clients (browser/devtools/extensions) may repeatedly attempt reconnects; avoid log spam on /ws.
			if r.URL.Path != "/ws" {
				log.Printf("Auth Error [%s]: No token provided (path=%s remote=%s)\n", source, r.URL.Path, r.RemoteAddr)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Remove "Bearer " prefix if present
		if len(idToken) > 7 && idToken[:7] == "Bearer " {
			idToken = idToken[7:]
		}

		// Final check after stripping
		if idToken == "" || idToken == "undefined" || idToken == "null" {
			// WebSocket clients (browser/devtools/extensions) may repeatedly attempt reconnects; avoid log spam on /ws.
			if r.URL.Path != "/ws" {
				log.Printf("Auth Error [%s]: Token value is invalid (path=%s remote=%s value=%q)\n", source, r.URL.Path, r.RemoteAddr, idToken)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Attempting verification [%s] (path=%s remote=%s len=%d start=%.10s...)\n", source, r.URL.Path, r.RemoteAddr, len(idToken), idToken)

		token, err := authClient.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			log.Printf("Firebase Verify Error [%s]: %v\n", source, err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Printf("Verified ID token for user: %v\n", token.UID)

		// Inject UID into context
		ctx := context.WithValue(r.Context(), uidKey, token.UID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {

	uid, ok := r.Context().Value(uidKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	mutex.Lock()
	clients[conn] = uid
	mutex.Unlock()

	log.Printf("Client connected: %s\n", uid)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {

			mutex.Lock()
			delete(clients, conn)
			mutex.Unlock()

			conn.Close()
			log.Println("Client disconnected")
			break
		}
	}
}

func broadcast(collection string, data bson.M) {

	mutex.Lock()
	defer mutex.Unlock()

	for conn, uid := range clients {
		shouldSend := false

		if collection == "numbers" {
			// If it's a new number, only send to the owner
			if data["userId"] == uid {
				shouldSend = true
			}
		} else if collection == "inbound_messages" {
			// If it's a new message, check if the recipient number belongs to this user
			to := data["to"].(string)

			// We need to check if 'to' is in the user's purchased numbers
			// Optimization: We could cache this, but for now we query DB
			count, _ := db.Collection("numbers").CountDocuments(context.TODO(), bson.M{
				"userId": uid,
				"phone":  to,
			})
			if count > 0 {
				shouldSend = true
			}
		} else if collection == "whitelist" {
			// If it's a new whitelist entry, only send to the owner
			if data["userId"] == uid {
				shouldSend = true
			}
		}

		if shouldSend {
			payload := map[string]interface{}{
				"collection": collection,
				"data":       data,
			}
			jsonData, _ := json.Marshal(payload)
			err := conn.WriteMessage(websocket.TextMessage, jsonData)
			if err != nil {
				conn.Close()
				delete(clients, conn)
			}
		}
	}
}

func watchMongoChanges() {

	stream, err := db.Watch(
		context.TODO(),
		mongo.Pipeline{},
		options.ChangeStream().SetFullDocument(options.UpdateLookup),
	)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Watching entire database...")

	for stream.Next(context.TODO()) {

		var event bson.M
		stream.Decode(&event)

		ns := event["ns"].(bson.M)
		collection := ns["coll"]

		fullDoc := event["fullDocument"]

		if fullDoc == nil {
			continue
		}

		doc := fullDoc.(bson.M)
		broadcast(collection.(string), doc)
	}
}

func handleTwilioInbound(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)

	if r.Method == "OPTIONS" {
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	from := r.FormValue("From")
	to := r.FormValue("To")
	body := r.FormValue("Text")

	if from != "" && from[0] != '+' {
		from = "+" + from
	}
	if to != "" && to[0] != '+' {
		to = "+" + to
	}

	log.Println("Inbound SMS:", from, to, body)

	inboundCollection := db.Collection("inbound_messages")

	doc := bson.M{
		"from":        from,
		"to":          to,
		"body":        body,
		"received_at": time.Now(),
	}

	_, err = inboundCollection.InsertOne(context.TODO(), doc)

	if err != nil {
		log.Println("Mongo insert error:", err)
	}

	w.WriteHeader(http.StatusOK)
}

func handleSyncUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(uidKey).(string)
	if !ok {
		log.Println("SyncUser Error: No UID found in context (Unauthorized)")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	log.Printf("SyncUser: Received sync request for UID: %s\n", uid)
	usersCollection := db.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if user already exists
	var existingUser bson.M
	err := usersCollection.FindOne(ctx, bson.M{"uid": uid}).Decode(&existingUser)

	if err == mongo.ErrNoDocuments {
		// User doesn't exist, create a new record
		newUser := bson.M{
			"uid":        uid,
			"created_at": time.Now(),
		}
		_, err := usersCollection.InsertOne(ctx, newUser)
		if err != nil {
			log.Printf("Error creating user: %v\n", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
		log.Printf("New user record created for: %s\n", uid)
	} else if err != nil {
		log.Printf("Error querying user: %v\n", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
func handleInitialData(w http.ResponseWriter, r *http.Request) {

	uid, ok := r.Context().Value(uidKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	inboundCollection := db.Collection("inbound_messages")
	numbersCollection := db.Collection("numbers")

	ctx := context.TODO()

	// 1. Fetch numbers owned by this user
	numbersCursor, _ := numbersCollection.Find(ctx, bson.M{"userId": uid})
	var numbers []bson.M
	numbersCursor.All(ctx, &numbers)

	// 2. Extract phone numbers to filter inbound messages
	var userPhoneNumbers []string
	for _, n := range numbers {
		if phone, ok := n["phone"].(string); ok {
			userPhoneNumbers = append(userPhoneNumbers, phone)
		}
	}

	// 3. Fetch inbound messages sent to the user's numbers
	var inbound []bson.M
	if len(userPhoneNumbers) > 0 {
		inboundCursor, _ := inboundCollection.Find(ctx, bson.M{
			"to": bson.M{"$in": userPhoneNumbers},
		})
		inboundCursor.All(ctx, &inbound)
	}

	response := map[string]interface{}{
		"inbound_messages": inbound,
		"numbers":          numbers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleWhitelist(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(uidKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	whitelistCollection := db.Collection("whitelist")
	ctx := context.TODO()

	switch r.Method {
	case "GET":
		cursor, err := whitelistCollection.Find(ctx, bson.M{"userId": uid})
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		var list []bson.M
		cursor.All(ctx, &list)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)

	case "POST":
		var body struct {
			Phone string `json:"phone"`
		}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		phone := body.Phone
		if phone == "" {
			http.Error(w, "Phone number is required", http.StatusBadRequest)
			return
		}

		// Normalize phone
		if phone[0] != '+' {
			phone = "+" + phone
		}

		// Basic E.164 validation: + followed by 7 to 15 digits
		phoneRegex := regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
		if !phoneRegex.MatchString(phone) {
			http.Error(w, "Invalid phone number format. Use E.164 format (e.g., +1234567890).", http.StatusBadRequest)
			return
		}

		// Check if already exists
		count, _ := whitelistCollection.CountDocuments(ctx, bson.M{"userId": uid, "phone": phone})
		if count > 0 {
			w.WriteHeader(http.StatusOK) // Already whitelisted
			return
		}

		doc := bson.M{
			"userId":     uid,
			"phone":      phone,
			"created_at": time.Now(),
		}

		_, err = whitelistCollection.InsertOne(ctx, doc)
		if err != nil {
			http.Error(w, "Failed to add to whitelist", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
