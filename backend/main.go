package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
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
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, DELETE")
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
	http.Handle("/analytics", AuthMiddleware(http.HandlerFunc(handleAnalytics)))

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

	case "DELETE":
		phone := r.URL.Query().Get("phone")
		if phone == "" {
			http.Error(w, "Phone number is required", http.StatusBadRequest)
			return
		}

		_, err := whitelistCollection.DeleteOne(ctx, bson.M{"userId": uid, "phone": phone})
		if err != nil {
			http.Error(w, "Failed to remove from whitelist", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleAnalytics(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(uidKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	numbersCollection := db.Collection("numbers")
	inboundCollection := db.Collection("inbound_messages")
	ctx := context.TODO()

	// Get filter parameters
	daysStr := r.URL.Query().Get("days")
	days := 7 // Default to 7 days
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
		days = d
	}
	targetPhone := r.URL.Query().Get("phone")

	// 1. Get user's numbers
	numbersCursor, _ := numbersCollection.Find(ctx, bson.M{"userId": uid})
	var numbers []bson.M
	numbersCursor.All(ctx, &numbers)

	var userPhoneNumbers []string
	var availableNumbers []string
	for _, n := range numbers {
		if phone, ok := n["phone"].(string); ok {
			userPhoneNumbers = append(userPhoneNumbers, phone)
			availableNumbers = append(availableNumbers, phone)
		}
	}

	// Filter userPhoneNumbers if a target phone is specified
	if targetPhone != "" {
		found := false
		for _, p := range userPhoneNumbers {
			if p == targetPhone {
				found = true
				break
			}
		}
		if found {
			userPhoneNumbers = []string{targetPhone}
		} else {
			// If filtered number doesn't belong to user, return empty but with number list
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"volume_chart":      []interface{}{},
				"sender_stats":      map[string]int{"total": 0, "unique": 0, "repeated": 0},
				"top_senders":       []interface{}{},
				"top_numbers":       []interface{}{},
				"available_numbers": availableNumbers,
			})
			return
		}
	}

	if len(userPhoneNumbers) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"volume_chart":      []interface{}{},
			"sender_stats":      map[string]int{"total": 0, "unique": 0, "repeated": 0},
			"top_senders":       []interface{}{},
			"top_numbers":       []interface{}{},
			"available_numbers": availableNumbers,
		})
		return
	}

	// Calculate start time based on 'days'
	startTime := time.Now().AddDate(0, 0, -days)

	// Base filter for messages
	matchFilter := bson.M{
		"to":          bson.M{"$in": userPhoneNumbers},
		"received_at": bson.M{"$gte": startTime},
	}

	// 2. Aggregate Volume by Day
	volumePipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$received_at"}},
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	volumeCursor, _ := inboundCollection.Aggregate(ctx, volumePipeline)
	volumeData := []bson.M{}
	volumeCursor.All(ctx, &volumeData)

	// 3. Sender Statistics (Unique vs Repeated)
	senderPipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$from",
			"count": bson.M{"$sum": 1},
		}}},
	}
	senderCursor, _ := inboundCollection.Aggregate(ctx, senderPipeline)
	senders := []bson.M{}
	senderCursor.All(ctx, &senders)

	unique := 0
	repeated := 0
	for _, s := range senders {
		countValue := s["count"]
		var count int64
		switch v := countValue.(type) {
		case int32:
			count = int64(v)
		case int64:
			count = v
		}

		if count > 1 {
			repeated++
		} else {
			unique++
		}
	}

	// 4. Top Senders
	topSendersPipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$from",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: 5}},
	}
	topSendersCursor, _ := inboundCollection.Aggregate(ctx, topSendersPipeline)
	topSenders := []bson.M{}
	topSendersCursor.All(ctx, &topSenders)

	// 5. Messages per Seeded Number
	perNumPipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$to",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	perNumCursor, _ := inboundCollection.Aggregate(ctx, perNumPipeline)
	perNum := []bson.M{}
	perNumCursor.All(ctx, &perNum)

	response := map[string]interface{}{
		"volume_chart": volumeData,
		"sender_stats": map[string]int{
			"total":    len(senders),
			"unique":   unique,
			"repeated": repeated,
		},
		"top_senders":       topSenders,
		"top_numbers":       perNum,
		"available_numbers": availableNumbers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
