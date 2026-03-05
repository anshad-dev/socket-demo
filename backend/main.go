package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	clients = make(map[*websocket.Conn]bool)
	mutex   = sync.Mutex{}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	client *mongo.Client
	db     *mongo.Database
)

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func main() {

	connectMongo()

	go watchMongoChanges()

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/twilio/inbound", handleTwilioInbound)
	http.HandleFunc("/initial-data", handleInitialData)

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

func handleWebSocket(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	log.Println("Client connected")

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

func broadcast(data []byte) {

	mutex.Lock()
	defer mutex.Unlock()

	for client := range clients {

		err := client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			client.Close()
			delete(clients, client)
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

		payload := map[string]interface{}{
			"collection": collection,
			"data":       fullDoc,
		}

		log.Println("Change detected:", payload)

		jsonData, _ := json.Marshal(payload)

		broadcast(jsonData)
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
	body := r.FormValue("Body")

	log.Println("Inbound SMS:", from, to, body)

	inboundCollection := db.Collection("inbound_messages")

	doc := bson.M{
		"from": from,
		"to":   to,
		"body": body,
	}

	_, err = inboundCollection.InsertOne(context.TODO(), doc)

	if err != nil {
		log.Println("Mongo insert error:", err)
	}

	w.WriteHeader(http.StatusOK)
}
func handleInitialData(w http.ResponseWriter, r *http.Request) {

	enableCors(&w)

	if r.Method == "OPTIONS" {
		return
	}

	inboundCollection := db.Collection("inbound_messages")
	numbersCollection := db.Collection("numbers")

	ctx := context.TODO()

	inboundCursor, _ := inboundCollection.Find(ctx, bson.M{})
	var inbound []bson.M
	inboundCursor.All(ctx, &inbound)

	numbersCursor, _ := numbersCollection.Find(ctx, bson.M{})
	var numbers []bson.M
	numbersCursor.All(ctx, &numbers)

	response := map[string]interface{}{
		"inbound_messages": inbound,
		"numbers":          numbers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
