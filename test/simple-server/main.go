package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("simple-server running on port 8080")
	err := http.ListenAndServe(":8080", http.HandlerFunc(handler))
	if err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("hello world"))
}
