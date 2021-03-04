package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/streadway/amqp"
)

var (
	corrMap map[string]bool
	reqChan chan reqMsg
)

type reqMsg struct {
	NAME   string `json:"name"`
	CORRID string `json:"corrId"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func initRPCClient() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"",    // name
		true,  // durable
		true,  // delete when unused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	for req := range reqChan {
		err = ch.Publish(
			"",          // exchange
			"rpc_queue", // routing key
			false,       // mandatory
			false,       // immediate
			amqp.Publishing{
				ContentType:   "text/plain",
				CorrelationId: req.CORRID,
				ReplyTo:       q.Name,
				Body:          []byte(req.NAME),
			})
		failOnError(err, "Failed to publish a message")

		for d := range msgs {
			if corrMap[d.CorrelationId] {
				res := string(d.Body)
				go func() { respChan <- res }()
				d.Ack(false)
				delete(corrMap, req.CORRID)
				fmt.Printf("RPC Response: %s\n", res)
				break
			}
		}
	}

}

var respChan chan string

func main() {

	go initRPCClient()

	router := mux.NewRouter()
	router.Use(JSONMiddleware)

	// All URLs will be handled by this function
	corrMap = make(map[string]bool)
	reqChan = make(chan reqMsg, 250)
	respChan = make(chan string, 250)
	router.HandleFunc("/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := mux.Vars(r)["name"]

		corrID := randomString(32)
		corrMap[corrID] = true
		reqChan <- reqMsg{
			name,
			corrID,
		}
		w.Write([]byte(<-respChan))

	}).Methods("GET")
	fmt.Println("Server Started ✅")
	http.ListenAndServe(":8080", router)
}

//JSONMiddleware for requests..
func JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Header().Add("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
