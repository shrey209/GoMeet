package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Room struct {
	RoomID string
	Users  []string
}

var rooms = make(map[string]*Room)
var users = make(map[string]string)
var mutex = &sync.Mutex{}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleConnections)
	handler := corsMiddleware(mux)

	fmt.Println("Server started on :3001")
	if err := http.ListenAndServe(":3001", handler); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {

	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}
	defer socket.Close()

	socketID := socket.RemoteAddr().String()
	fmt.Println("A user connected:", socketID)

	defer func() {
		handleDisconnect(socketID)
	}()

	for {

		var message map[string]interface{}
		err := socket.ReadJSON(&message)
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}

		event := message["event"].(string)
		data := message["data"].(map[string]interface{})

		switch event {
		case "join":
			handleJoin(socketID, data)
		case "localDescription":
			handleLocalDescription(socket, socketID, data)
		case "remoteDescription":
			handleRemoteDescription(socket, socketID, data)
		case "iceCandidate":
			handleIceCandidate(socket, socketID, data)
		case "iceCandidateReply":
			handleIceCandidateReply(socket, socketID, data)
		}
	}
}

func handleJoin(socketID string, data map[string]interface{}) {
	roomID := data["roomId"].(string)

	mutex.Lock()
	defer mutex.Unlock()

	users[socketID] = roomID

	if _, exists := rooms[roomID]; !exists {
		rooms[roomID] = &Room{
			RoomID: roomID,
			Users:  []string{},
		}
	}

	rooms[roomID].Users = append(rooms[roomID].Users, socketID)
	fmt.Println("User added to room:", roomID)
}

func handleDisconnect(socketID string) {
	mutex.Lock()
	defer mutex.Unlock()

	roomID := users[socketID]
	if room, exists := rooms[roomID]; exists {

		newUsers := []string{}
		for _, user := range room.Users {
			if user != socketID {
				newUsers = append(newUsers, user)
			}
		}
		room.Users = newUsers
	}

	delete(users, socketID)
	fmt.Println("User disconnected:", socketID)
}

func handleLocalDescription(socket *websocket.Conn, socketID string, data map[string]interface{}) {
	description := data["description"].(string)
	broadcastToRoom(socketID, "localDescription", map[string]interface{}{
		"description": description,
	})
}

func handleRemoteDescription(socket *websocket.Conn, socketID string, data map[string]interface{}) {
	description := data["description"].(string)
	broadcastToRoom(socketID, "remoteDescription", map[string]interface{}{
		"description": description,
	})
}

func handleIceCandidate(socket *websocket.Conn, socketID string, data map[string]interface{}) {
	candidate := data["candidate"].(string)
	broadcastToRoom(socketID, "iceCandidate", map[string]interface{}{
		"candidate": candidate,
	})
}

func handleIceCandidateReply(socket *websocket.Conn, socketID string, data map[string]interface{}) {
	candidate := data["candidate"].(string)
	broadcastToRoom(socketID, "iceCandidateReply", map[string]interface{}{
		"candidate": candidate,
	})
}

func broadcastToRoom(socketID, event string, data map[string]interface{}) {
	mutex.Lock()
	defer mutex.Unlock()

	roomID := users[socketID]
	if room, exists := rooms[roomID]; exists {
		for _, user := range room.Users {
			if user != socketID {
				fmt.Printf("Broadcasting to %s: event=%s, data=%v\n", user, event, data)
			}
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
