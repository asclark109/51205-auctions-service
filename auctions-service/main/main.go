package main

import (
	"auctions-service/common"
	"auctions-service/domain"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"              // acquired by doing 'go get github.com/gorilla/mux.git'
	_ "github.com/lib/pq"                 // postgres
	amqp "github.com/rabbitmq/amqp091-go" // acquired by doing 'go get github.com/rabbitmq/amqp091-go'
)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the HomePage!")
	fmt.Println("Endpoint Hit: homePage")
}

func cancelAuction(auctionservice *AuctionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		itemId := vars["itemId"]

		var requestBody RequestStopAuction // parse request into a struct with assumed structure
		err := json.NewDecoder(r.Body).Decode(&requestBody)

		var response ResponseStopAuction

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response.Msg = "request body was ill-formed"

			json.NewEncoder(w).Encode(response)
			return
		}

		requesterUserId := requestBody.RequesterUserId
		cancelAuctionOutcome := auctionservice.CancelAuction(itemId, requesterUserId)

		if cancelAuctionOutcome == auctionNotExist {
			response.Msg = "auction does not exist."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if cancelAuctionOutcome == auctionCancellationRequesterIsNotSeller {
			response.Msg = "requesting user is not the seller of the item in auction. Not allowed to cancel auction."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if cancelAuctionOutcome == auctionAlreadyFinalized {
			response.Msg = "auction is already finalized (archived)."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if cancelAuctionOutcome == auctionAlreadyOver {
			response.Msg = "auction is already over."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if cancelAuctionOutcome == auctionAlreadyCanceled {
			response.Msg = "auction has already been canceled."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return

		}

		// success
		if cancelAuctionOutcome == auctionSuccessfullyCanceled {
			response.Msg = "successfully stopped auction."
			json.NewEncoder(w).Encode(response)
			return
		}

		panic("see cancelAuction() in main.go; could not determine an outcome for cancel Auction request")

	}
}

func getItemsUserHasBidsOn(auctionservice *AuctionService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// var res itemIds
		vars := mux.Vars(r)
		userId := vars["userId"]
		// fmt.Println(userId)
		itemIds := auctionservice.GetItemsUserHasBidsOn(userId)
		// fmt.Println(itemIds)

		response := ResponseGetItemsByUserId{*itemIds}

		js, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// json.NewEncoder(w).Encode(article)

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}

func getActiveAuctions(auctionservice *AuctionService) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		activeAuctions := auctionservice.GetActiveAuctions()

		exportedAuctions := make([]JsonAuction, len(*activeAuctions))
		for i, activeAuction := range *activeAuctions {
			exportedAuctions[i] = *ExportAuction(activeAuction)
		}

		response := ResponseGetActiveAuctions{exportedAuctions}

		js, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}

}

func stopAuction(auctionservice *AuctionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		itemId := vars["itemId"]

		// var requestBody RequestStopAuction // parse request into a struct with assumed structure
		var response ResponseStopAuction

		w.Header().Set("Content-Type", "application/json")

		stopAuctionOutcome := auctionservice.StopAuction(itemId)

		if stopAuctionOutcome == auctionNotExist {
			response.Msg = "auction does not exist."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if stopAuctionOutcome == auctionAlreadyFinalized {
			response.Msg = "auction is already finalized (archived)."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if stopAuctionOutcome == auctionAlreadyCanceled {
			response.Msg = "auction has already been canceled."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return

		}

		if stopAuctionOutcome == auctionAlreadyOver {
			response.Msg = "auction is already over."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		// success
		if stopAuctionOutcome == auctionSuccessfullyStopped {
			response.Msg = "successfully stopped auction."
			json.NewEncoder(w).Encode(response)
			return
		}

		panic("see stopAuction() in main.go; could not determine an outcome for stop Auction request")

	}
}

func createAuction(auctionservice *AuctionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// var res itemIds
		// vars := mux.Vars(r)
		// itemId := vars["itemId"]

		var requestBody RequestCreateAuction // parse request into a struct with assumed structure
		err := json.NewDecoder(r.Body).Decode(&requestBody)

		log.Println("[main] [.] create auction: ", requestBody)
		var response ResponseCreateAuction

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response.Msg = "request body was ill-formed"
			json.NewEncoder(w).Encode(response)
			return
		}

		itemId := requestBody.ItemId
		sellerUserId := requestBody.SellerUserId
		startTime, err1 := common.InterpretTimeStr(requestBody.StartTime)
		endTime, err2 := common.InterpretTimeStr(requestBody.EndTime)
		startPriceInCents := requestBody.StartPriceInCents

		if err1 != nil || err2 != nil {
			response.Msg = "startTime or endTime was not given in expected format: use YYYY-MM-DD HH:MM:SS.SSSSSS"
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		createAuctionOutcome := auctionservice.CreateAuction(itemId, sellerUserId, startTime, endTime, startPriceInCents)

		if createAuctionOutcome == auctionAlreadyCreated {
			response.Msg = "an auction already exists for this item."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if createAuctionOutcome == auctionStartsInPast {
			response.Msg = "auction would start in the past."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if createAuctionOutcome == auctionWouldStartTooSoon {
			response.Msg = "an auction cannot be created within 1 minute before auction start. schedule the auction for a later time."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if createAuctionOutcome == badTimeSpecified {
			response.Msg = "startTime is not < endTime."
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		// success
		if createAuctionOutcome == auctionSuccessfullyCreated {
			response.Msg = "successfully created auction."
			// w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		panic("see createAuction() in main.go; could not determine an outcome for create Auction request")

	}
}

func createNewBid(auctionservice *AuctionService, ch *amqp.Channel, newBidExchangeName, newBidQueueName string) http.HandlerFunc {

	// // // create channel with rabbitMQ;
	// ch, err := conn.Channel()
	// failOnError(err, "Failed to open a channel")
	// defer ch.Close()

	// declare exchange
	err := ch.ExchangeDeclare(
		newBidExchangeName, // name
		"fanout",           // type CHANGE TO FANOUT IF REFACTORING TO INCLUDE REAL-TIME-VIEWS
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)
	failOnError(err, "Failed to declare exchange: "+newBidExchangeName)

	// declare queue for us to send messages to
	_, err = ch.QueueDeclare(
		newBidQueueName, // name
		true,            // durable ORIGINALLY FALSE
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)
	failOnError(err, "Failed to declare queue: "+newBidQueueName)

	err = ch.QueueBind(
		newBidQueueName,    // queue name; ORIGINALLY q.Name
		"",                 // routing key
		newBidExchangeName, // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind a queue")

	return func(w http.ResponseWriter, r *http.Request) {
		// var res itemIds
		// vars := mux.Vars(r)
		// itemId := vars["itemId"]

		var requestBody RequestCreateNewBid // parse request into a struct with assumed structure
		err := json.NewDecoder(r.Body).Decode(&requestBody)

		log.Println("[main] [.] create new bid: ", requestBody)
		var response ResponseCreateNewBid

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			response.Msg = "request body was ill-formed"

			json.NewEncoder(w).Encode(response)
			return
		}

		// PENDING   AuctionState = "PENDING" // has not yet started
		// ACTIVE    AuctionState = "ACTIVE"  // is happening now
		// CANCELED  AuctionState = "CANCELED"
		// OVER      AuctionState = "OVER"      // is over (but winner has not been declared and auction has not been "archived away")
		// FINALIZED AuctionState = "FINALIZED" // is over and archived away; can delete
		// UNKNOWN   AuctionState = "UKNOWN"

		itemId := requestBody.ItemId
		bidderUserId := requestBody.BidderUserId
		timeReceived := time.Now()
		amountInCents := requestBody.AmountInCents

		if amountInCents < 0 {
			response.Msg = "bid money amount was negative integer."
			// response.WasNewTopBid = false
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		auctionState, isAcceptableBid := auctionservice.ValidateBid(itemId, bidderUserId, timeReceived, amountInCents) // rarely requests lock

		if isAcceptableBid {
			// publish new Bid to RabbitMQ; will process bid later

			rawBidData := domain.NewRawBidData(itemId, bidderUserId, timeReceived, amountInCents)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			body, err := json.Marshal(rawBidData)
			// fmt.Println(body)
			failOnError(err, "[main] Error encoding JSON")

			// log.Printf("[AlertEngine] Sending Auction data to RabbitMQ (item_id=%s)\n", auctionData.Item.ItemId)
			// log.Printf("[AlertEngine] Sending Auction data to RabbitMQ: %s\n", body)
			err = ch.PublishWithContext(ctx,
				newBidExchangeName, // exchange
				newBidQueueName,    // routing key WITH QUEUE q.Name
				false,              // mandatory
				false,              // immediate
				amqp.Publishing{
					ContentType:  "application/json",
					Body:         body,
					DeliveryMode: amqp.Persistent,
				})
			// amqp.Publishing{
			// 	ContentType: "text/plain",
			// 	Body:        []byte(body),
			// })
			// failOnError(err, "[main] Failed to publish a message")
			if err != nil {
				log.Println(err)
				response.Msg = "bid was well-formed, but system failed to save bid (likely, system was disconnected from RabbitMQ)"
				// response.WasNewTopBid = false
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(response)
				return
			}

			// else, success
			log.Printf("[main] [x] Sent validated bid data to RabbitMQ\n")
			response.Msg = fmt.Sprintf("system has received your bid at %s [UTC]", common.TimeToSQLTimestamp6(timeReceived))
			json.NewEncoder(w).Encode(response)
			return
		}

		// else the bid is not acceptable for some reason
		if auctionState == domain.UNKNOWN {
			response.Msg = "auction does not exist."
			// response.WasNewTopBid = false
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if auctionState == domain.PENDING {
			response.Msg = "auction has not yet started."
			// response.WasNewTopBid = false
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if auctionState == domain.OVER {
			response.Msg = "auction is already over."
			// response.WasNewTopBid = false
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		if auctionState == domain.FINALIZED {
			response.Msg = "auction has already been finalized (archived)."
			// response.WasNewTopBid = false
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}

		// if auctionState == domain.ACTIVE && !wasNewTopBid {
		// 	response.Msg = "bid was not a new top bid because it was under start price or under the current top bid price."
		// 	response.WasNewTopBid = false
		// 	w.WriteHeader(http.StatusBadRequest)
		// 	json.NewEncoder(w).Encode(response)
		// 	return
		// }

		// // success case 2
		// if auctionState == domain.ACTIVE && wasNewTopBid {
		// 	response.Msg = "successfully processed bid; bid was new top bid!"
		// 	response.WasNewTopBid = true
		// 	json.NewEncoder(w).Encode(response)
		// 	return
		// }

		panic("see createNewBid() in main.go; could not determine an outcome for request to place new Bid")

	}
}

// func processNewBid(auctionservice *AuctionService) http.HandlerFunc {

// 	return func(w http.ResponseWriter, r *http.Request) {
// 		// var res itemIds
// 		// vars := mux.Vars(r)
// 		// itemId := vars["itemId"]

// 		var requestBody RequestProcessNewBid // parse request into a struct with assumed structure
// 		err := json.NewDecoder(r.Body).Decode(&requestBody)

// 		if err != nil {
// 			return
// 		}

// 		// fmt.Println(requestBody)
// 		log.Println("[main] [.] process bid: ", requestBody)
// 		// var response ResponseProcessNewBid

// 		// w.Header().Set("Content-Type", "application/json")
// 		// if err != nil {
// 		// 	w.WriteHeader(http.StatusBadRequest)
// 		// 	response.Msg = "request body was ill-formed"

// 		// 	json.NewEncoder(w).Encode(response)
// 		// 	return
// 		// }

// 		itemId := requestBody.ItemId
// 		bidderUserId := requestBody.BidderUserId
// 		timeReceived, _ := common.InterpretTimeStr(requestBody.TimeReceived)
// 		amountInCents := requestBody.AmountInCents

// 		if amountInCents < 0 {
// 			msg := "bid money amount was negative integer."
// 			// response.WasNewTopBid = false
// 			// w.WriteHeader(http.StatusBadRequest)
// 			// json.NewEncoder(w).Encode(response)
// 			log.Println(msg)
// 			return
// 		}

// 		auctionInteractionOutcome, auctionState, wasNewTopBid := auctionservice.ProcessNewBid(itemId, bidderUserId, *timeReceived, amountInCents)

// 		if auctionInteractionOutcome == auctionNotExist {
// 			msg := "auction does not exist."
// 			log.Println(msg)
// 			return
// 		}

// 		if auctionState == domain.PENDING {
// 			msg := "auction has not yet started."
// 			log.Println(msg)
// 			return
// 		}

// 		if auctionState == domain.OVER {
// 			msg := "auction is already over."
// 			log.Println(msg)
// 			return
// 		}

// 		if auctionState == domain.FINALIZED {
// 			msg := "auction has already been finalized (archived)."
// 			log.Println(msg)
// 			return
// 		}

// 		if auctionState == domain.ACTIVE && !wasNewTopBid {
// 			msg := "bid was not a new top bid because it was under start price or under the current top bid price."
// 			log.Println(msg)
// 			return
// 		}

// 		// success case 2
// 		if auctionState == domain.ACTIVE && wasNewTopBid {
// 			msg := "successfully processed bid; bid was new top bid!"
// 			log.Println(msg)
// 			return
// 		}

// 		panic("see processNewBid() in main.go; could not determine an outcome for place new Bid request")

// 	}
// }

// func createAndProcessNewBid(auctionservice *AuctionService) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		// var res itemIds
// 		// vars := mux.Vars(r)
// 		// itemId := vars["itemId"]

// 		var requestBody RequestProcessNewBid // parse request into a struct with assumed structure
// 		err := json.NewDecoder(r.Body).Decode(&requestBody)

// 		fmt.Println(requestBody)
// 		var response ResponseProcessNewBid

// 		w.Header().Set("Content-Type", "application/json")
// 		if err != nil {
// 			w.WriteHeader(http.StatusBadRequest)
// 			response.Msg = "request body was ill-formed"

// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		itemId := requestBody.ItemId
// 		bidderUserId := requestBody.BidderUserId
// 		timeReceived := time.Now()
// 		amountInCents := requestBody.AmountInCents

// 		if amountInCents < 0 {
// 			response.Msg = "bid money amount was negative integer."
// 			response.WasNewTopBid = false
// 			w.WriteHeader(http.StatusBadRequest)
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		auctionInteractionOutcome, auctionState, wasNewTopBid := auctionservice.ProcessNewBid(itemId, bidderUserId, timeReceived, amountInCents)

// 		if auctionInteractionOutcome == auctionNotExist {
// 			response.Msg = "auction does not exist."
// 			response.WasNewTopBid = false
// 			w.WriteHeader(http.StatusBadRequest)
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		if auctionState == domain.PENDING {
// 			response.Msg = "auction has not yet started."
// 			response.WasNewTopBid = false
// 			w.WriteHeader(http.StatusBadRequest)
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		if auctionState == domain.OVER {
// 			response.Msg = "auction is already over."
// 			response.WasNewTopBid = false
// 			w.WriteHeader(http.StatusBadRequest)
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		if auctionState == domain.FINALIZED {
// 			response.Msg = "auction has already been finalized (archived)."
// 			response.WasNewTopBid = false
// 			w.WriteHeader(http.StatusBadRequest)
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		if auctionState == domain.ACTIVE && !wasNewTopBid {
// 			response.Msg = "bid was not a new top bid because it was under start price or under the current top bid price."
// 			response.WasNewTopBid = false
// 			w.WriteHeader(http.StatusBadRequest)
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		// success case 2
// 		if auctionState == domain.ACTIVE && wasNewTopBid {
// 			response.Msg = "successfully processed bid; bid was new top bid!"
// 			response.WasNewTopBid = true
// 			json.NewEncoder(w).Encode(response)
// 			return
// 		}

// 		panic("see processNewBid() in main.go; could not determine an outcome for place new Bid request")

// 	}
// }

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

// func publishNotif(w http.ResponseWriter, r *http.Request) {
// 	// make connection
// 	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq-server:5672/")
// 	failOnError(err, "Failed to connect to RabbitMQ")
// 	defer conn.Close()

// 	// create a channel
// 	ch, err := conn.Channel()
// 	failOnError(err, "Failed to open a channel")
// 	defer ch.Close()

// 	// declare queue for us to send messages to
// 	q, err := ch.QueueDeclare(
// 		"notifications", // name
// 		false,           // durable
// 		false,           // delete when unused
// 		false,           // exclusive
// 		false,           // no-wait
// 		nil,             // arguments
// 	)
// 	failOnError(err, "Failed to declare a queue")
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	body := "Hello World!"
// 	err = ch.PublishWithContext(ctx,
// 		"",     // exchange
// 		q.Name, // routing key
// 		false,  // mandatory
// 		false,  // immediate
// 		amqp.Publishing{
// 			ContentType: "text/plain",
// 			Body:        []byte(body),
// 		})
// 	failOnError(err, "Failed to publish a message")
// 	log.Printf(" [x] Sent %s\n", body)
// }

// method that when executed spawns a goroutine to listen for incoming
// messages on a queue for new bids. With each new bid that appears
// in the queue, this method calls upon the auctionservice to process
// the new bid
func handleNewBids(auctionservice *AuctionService, conn *amqp.Connection, newBidExchangeName, newBidQueueName string) {

	// msgs, err := ch.Consume(
	// 	q.Name, // queue
	// 	"",     // consumer
	// 	true,   // auto-ack
	// 	false,  // exclusive
	// 	false,  // no-local
	// 	false,  // no-wait
	// 	nil,    // args
	// )
	// newBidExchangeName := "auction.new-bid"
	// newBidQueueName := "auction.process-bid"

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		newBidExchangeName, // name
		"fanout",           // type
		true,               // durable
		false,              // auto-deleted
		false,              // internal
		false,              // no-wait
		nil,                // arguments
	)
	failOnError(err, "Failed to declare exchange: "+newBidExchangeName)

	_, err = ch.QueueDeclare(
		newBidQueueName, // name
		true,            // durable ORIGINALLY FALSE
		false,           // delete when unused
		false,           // exclusive
		false,           // no-wait
		nil,             // arguments
	)
	failOnError(err, "Failed to declare queue: "+newBidQueueName)

	// fmt.Printf("q.Name: %s\n", q.Name)
	msgs, err := ch.Consume(
		newBidQueueName, // queue ORIGINALLY q.Name
		"",              // consumer
		true,            // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	failOnError(err, "Failed to register a consumer")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("[main] [.] received bid data to process: %s", d.Body)
			// characterize

			var rawBidData domain.RawBidData
			// json.NewDecoder(bytes.NewBuffer(d.Body)).Decode(&rawBidData)
			err := json.Unmarshal(d.Body, &rawBidData)
			failOnError(err, "[main] encountered problem unmarshalling raw bid data")
			itemId := rawBidData.ItemId
			bidderUserId := rawBidData.BidderUserId
			timeReceived, _ := common.InterpretTimeStr(rawBidData.TimeReceived)
			amountInCents := rawBidData.AmountInCents
			// fmt.Print("got: ", itemId, bidderUserId, timeReceived, amountInCents)
			if amountInCents < 0 {
				msg := "bid money amount was negative integer."
				// response.WasNewTopBid = false
				// w.WriteHeader(http.StatusBadRequest)
				// json.NewEncoder(w).Encode(response)
				log.Println(msg)
				return
			}

			auctionInteractionOutcome, auctionState, wasNewTopBid := auctionservice.ProcessNewBid(itemId, bidderUserId, *timeReceived, amountInCents)

			var msg string
			switch {
			case auctionInteractionOutcome == auctionNotExist:
				msg = "[main] auction does not exist."
			case auctionState == domain.PENDING:
				msg = "[main] auction has not yet started."
			case auctionState == domain.OVER:
				msg = "[main] auction is already over."
			case auctionState == domain.FINALIZED:
				msg = "[main] auction has already been finalized (archived)."
			case auctionState == domain.ACTIVE && !wasNewTopBid:
				msg = "[main] bid was not a new top bid because it was under start price or under the current top bid price."
			case auctionState == domain.ACTIVE && wasNewTopBid:
				msg = "[main] successfully processed bid; bid was new top bid!"
			default:
				panic("[main] error! see main.go handleNewBids(); reached end of method without understanding case")
			}
			log.Println(msg)

		}
	}()

	log.Printf("[handleNewBids] [*] Waiting for RabbitMQ messages. To exit press CTRL+C")
	<-forever
}

func handleItemCounterfeit(auctionservice *AuctionService, conn *amqp.Connection, ItemCounterfeitExchangeName, ItemCounterfeitQueueName string) {

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		ItemCounterfeitExchangeName, // name
		"fanout",                    // type
		true,                        // durable
		false,                       // auto-deleted
		false,                       // internal
		false,                       // no-wait
		nil,                         // arguments
	)
	failOnError(err, "Failed to declare exchange: "+ItemCounterfeitExchangeName)

	_, err = ch.QueueDeclare(
		ItemCounterfeitQueueName, // name
		true,                     // durable ORIGINALLY FALSE
		false,                    // delete when unused
		false,                    // exclusive
		false,                    // no-wait
		nil,                      // arguments
	)
	failOnError(err, "Failed to declare queue: "+ItemCounterfeitQueueName)

	err = ch.QueueBind(
		ItemCounterfeitQueueName,    // queue name; ORIGINALLY q.Name
		"",                          // routing key
		ItemCounterfeitExchangeName, // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind queue for ItemCounterfeit")

	// fmt.Printf("q.Name: %s\n", q.Name)
	msgs, err := ch.Consume(
		ItemCounterfeitQueueName, // queue ORIGINALLY q.Name
		"",                       // consumer
		true,                     // auto-ack
		false,                    // exclusive
		false,                    // no-local
		false,                    // no-wait
		nil,                      // args
	)
	failOnError(err, "Failed to register consumer for ItemCounterfeit")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("[main] [.] received Item.counterfeit event: %s", d.Body)
			// characterize

			var requestBody ItemCounterfeitEvent
			// json.NewDecoder(bytes.NewBuffer(d.Body)).Decode(&rawBidData)
			err := json.Unmarshal(d.Body, &requestBody)
			if err != nil {
				fmt.Println("[main] encountered problem unmarshalling Item.counterfeit event")
				// failOnError(err, "[main] encountered problem unmarshalling Item.counterfeit event")
			} else {

			}

			itemId := requestBody.ItemId
			fmt.Printf("[main] STUBBED reacting to Item.counterfeit event (itemId=%s)...\n", itemId)
			// auctionInteractionOutcome := auctionservice.StopAuction(itemId)

			// var msg string
			// switch {
			// case auctionInteractionOutcome == auctionNotExist:
			// 	msg = "[main] auction does not exist."
			// case auctionInteractionOutcome == auctionAlreadyFinalized:
			// 	msg = "[main] fail. auction is already finalized."
			// case auctionInteractionOutcome == auctionAlreadyCanceled: // should never happen
			// 	msg = "[main] fail. auction is already canceled."
			// case auctionInteractionOutcome == auctionAlreadyOver: // should never happen
			// 	msg = "[main] fail. auction is already over."
			// default:
			// 	panic("[main] error! see main.go handleItemCounterfeit(); reached end of method without understanding case")
			// }
			// log.Println(msg)
		}
	}()

	log.Printf("[handleItemCounterfeit] [*] Waiting for RabbitMQ messages. To exit press CTRL+C")
	<-forever
}

func handleItemInappropriate(auctionservice *AuctionService, conn *amqp.Connection, ItemInapropriateExchangeName, ItemInapropriateQueueName string) {

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		ItemInapropriateExchangeName, // name
		"fanout",                     // type
		true,                         // durable
		false,                        // auto-deleted
		false,                        // internal
		false,                        // no-wait
		nil,                          // arguments
	)
	failOnError(err, "Failed to declare exchange: "+ItemInapropriateExchangeName)

	_, err = ch.QueueDeclare(
		ItemInapropriateQueueName, // name
		true,                      // durable ORIGINALLY FALSE
		false,                     // delete when unused
		false,                     // exclusive
		false,                     // no-wait
		nil,                       // arguments
	)
	failOnError(err, "Failed to declare queue: "+ItemInapropriateQueueName)

	err = ch.QueueBind(
		ItemInapropriateQueueName,    // queue name; ORIGINALLY q.Name
		"",                           // routing key
		ItemInapropriateExchangeName, // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind queue for ItemCounterfeit")

	// fmt.Printf("q.Name: %s\n", q.Name)
	msgs, err := ch.Consume(
		ItemInapropriateQueueName, // queue ORIGINALLY q.Name
		"",                        // consumer
		true,                      // auto-ack
		false,                     // exclusive
		false,                     // no-local
		false,                     // no-wait
		nil,                       // args
	)
	failOnError(err, "Failed to register consumer for ItemInapropriate")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("[main] [.] received Item.Inapropriate event: %s", d.Body)
			// characterize

			var requestBody ItemInapropriateEvent
			// json.NewDecoder(bytes.NewBuffer(d.Body)).Decode(&rawBidData)
			err := json.Unmarshal(d.Body, &requestBody)
			if err != nil {
				fmt.Println("[main] encountered problem unmarshalling Item.inapropriate event")
				// failOnError(err, "[main] encountered problem unmarshalling Item.inapropriate event")
			} else {

			}

			itemId := requestBody.ItemId
			fmt.Printf("[main] STUBBED reacting to Item.inapropriate event (itemId=%s)...\n", itemId)
			// auctionInteractionOutcome := auctionservice.StopAuction(itemId)

			// var msg string
			// switch {
			// case auctionInteractionOutcome == auctionNotExist:
			// 	msg = "[main] auction does not exist."
			// case auctionInteractionOutcome == auctionAlreadyFinalized:
			// 	msg = "[main] fail. auction is already finalized."
			// case auctionInteractionOutcome == auctionAlreadyCanceled: // should never happen
			// 	msg = "[main] fail. auction is already canceled."
			// case auctionInteractionOutcome == auctionAlreadyOver: // should never happen
			// 	msg = "[main] fail. auction is already over."
			// default:
			// 	panic("[main] error! see main.go handleItemInapropriate(); reached end of method without understanding case")
			// }
			// log.Println(msg)
		}
	}()

	log.Printf("[handleItemInappropriate] [*] Waiting for RabbitMQ messages. To exit press CTRL+C")
	<-forever
}

func handleUserUpdate(auctionservice *AuctionService, conn *amqp.Connection, UserUpdateExchangeName, UserUpdateQueueName string) {
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		UserUpdateExchangeName, // name
		"fanout",               // type
		true,                   // durable
		false,                  // auto-deleted
		false,                  // internal
		false,                  // no-wait
		nil,                    // arguments
	)
	failOnError(err, "Failed to declare exchange: "+UserUpdateExchangeName)

	_, err = ch.QueueDeclare(
		UserUpdateQueueName, // name
		true,                // durable ORIGINALLY FALSE
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	failOnError(err, "Failed to declare queue: "+UserUpdateQueueName)

	err = ch.QueueBind(
		UserUpdateQueueName,    // queue name; ORIGINALLY q.Name
		"",                     // routing key
		UserUpdateExchangeName, // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind queue for ItemCounterfeit")

	// fmt.Printf("q.Name: %s\n", q.Name)
	msgs, err := ch.Consume(
		UserUpdateQueueName, // queue ORIGINALLY q.Name
		"",                  // consumer
		true,                // auto-ack
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,                 // args
	)
	failOnError(err, "Failed to register consumer for UserUpdate")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("[main] [.] received User.UserUpdate event: %s", d.Body)
			// characterize

			// var requestBody UserUpdateEvent
			// json.NewDecoder(bytes.NewBuffer(d.Body)).Decode(&rawBidData)
			// err := json.Unmarshal(d.Body, &requestBody)
			// if err != nil {
			// 	fmt.Println("[main] encountered problem unmarshalling Item.UserUpdate event")
			// 	// failOnError(err, "[main] encountered problem unmarshalling Item.inapropriate event")
			// }

			// itemId := requestBody.ItemId
			fmt.Printf("[main] STUBBED reacting to Item.UserUpdate event (userId=)...\n")
			// auctionInteractionOutcome := auctionservice.StopAuction(itemId)

			// var msg string
			// switch {
			// case auctionInteractionOutcome == auctionNotExist:
			// 	msg = "[main] auction does not exist."
			// case auctionInteractionOutcome == auctionAlreadyFinalized:
			// 	msg = "[main] fail. auction is already finalized."
			// case auctionInteractionOutcome == auctionAlreadyCanceled: // should never happen
			// 	msg = "[main] fail. auction is already canceled."
			// case auctionInteractionOutcome == auctionAlreadyOver: // should never happen
			// 	msg = "[main] fail. auction is already over."
			// default:
			// 	panic("[main] error! see main.go handleItemInapropriate(); reached end of method without understanding case")
			// }
			// log.Println(msg)
		}
	}()

	log.Printf("[handleUserUpdate] [*] Waiting for RabbitMQ messages. To exit press CTRL+C")
	<-forever
}

func handleUserDelete(auctionservice *AuctionService, conn *amqp.Connection, UserDeleteExchangeName, UserDeleteQueueName string) {

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		UserDeleteExchangeName, // name
		"fanout",               // type
		true,                   // durable
		false,                  // auto-deleted
		false,                  // internal
		false,                  // no-wait
		nil,                    // arguments
	)
	failOnError(err, "Failed to declare exchange: "+UserDeleteExchangeName)

	_, err = ch.QueueDeclare(
		UserDeleteQueueName, // name
		true,                // durable ORIGINALLY FALSE
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	failOnError(err, "Failed to declare queue: "+UserDeleteQueueName)

	err = ch.QueueBind(
		UserDeleteQueueName,    // queue name; ORIGINALLY q.Name
		"",                     // routing key
		UserDeleteExchangeName, // exchange
		false,
		nil,
	)
	failOnError(err, "Failed to bind queue for ItemCounterfeit")

	// fmt.Printf("q.Name: %s\n", q.Name)
	msgs, err := ch.Consume(
		UserDeleteQueueName, // queue ORIGINALLY q.Name
		"",                  // consumer
		true,                // auto-ack
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,                 // args
	)
	failOnError(err, "Failed to register consumer for UserDelete")

	var forever chan struct{}

	go func() {
		for d := range msgs {
			log.Printf("[main] [.] received User.UserDelete event: %s", d.Body)
			// characterize

			var requestBody UserDeleteEvent
			// json.NewDecoder(bytes.NewBuffer(d.Body)).Decode(&requestBody)
			err := json.Unmarshal(d.Body, &requestBody)
			if err != nil {
				fmt.Println("[main] encountered problem unmarshalling Item.UserUpdate event")
				// failOnError(err, "[main] encountered problem unmarshalling Item.inapropriate event")
			}

			userId := requestBody.UserId
			fmt.Printf("[main] reacting to Item.UserDelete event (userId=)...\n")
			numAuctionsWBidUpdates := auctionservice.DeactivateUserBids(userId)
			fmt.Printf("[main] deactivated %d bids \n", numAuctionsWBidUpdates)

			// var msg string
			// switch {
			// case auctionInteractionOutcome == auctionNotExist:
			// 	msg = "[main] auction does not exist."
			// case auctionInteractionOutcome == auctionAlreadyFinalized:
			// 	msg = "[main] fail. auction is already finalized."
			// case auctionInteractionOutcome == auctionAlreadyCanceled: // should never happen
			// 	msg = "[main] fail. auction is already canceled."
			// case auctionInteractionOutcome == auctionAlreadyOver: // should never happen
			// 	msg = "[main] fail. auction is already over."
			// default:
			// 	panic("[main] error! see main.go handleItemInapropriate(); reached end of method without understanding case")
			// }
			// log.Println(msg)
		}
	}()

	log.Printf("[handleUserDelete] [*] Waiting for RabbitMQ messages. To exit press CTRL+C")
	<-forever
}

func handleHTTPAPIRequests(auctionservice *AuctionService, conn *amqp.Connection, newBidExchangeName, newBidQueueName string) {
	// creates a new instance of a mux router
	myRouter := mux.NewRouter().StrictSlash(true)
	// replace http.HandleFunc with myRouter.HandleFunc
	// myRouter.HandleFunc("/", homePage)
	// myRouter.HandleFunc("/all", returnAllArticles)
	// myRouter.HandleFunc("/publishNotifc", publishNotif)
	// myRouter.HandleFunc("/rowsindb", getrowsindb(db))
	// finally, instead of passing in nil, we want
	// to pass in our newly created router as the second
	// argument

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")

	// define all REST/HTTP API endpoints below
	apiVersion := "v1"
	myRouter.HandleFunc("/", homePage)
	myRouter.HandleFunc(fmt.Sprintf("/api/%s/Auctions/", apiVersion), createAuction(auctionservice)).Methods("POST")
	// myRouter.HandleFunc(fmt.Sprintf("/api/%s/Bids/", apiVersion), createAndProcessNewBid(auctionservice, newBidExchangeName, newBidQueueName)).Methods("POST") ORIGINAL; did everything
	myRouter.HandleFunc(fmt.Sprintf("/api/%s/Bids/", apiVersion), createNewBid(auctionservice, ch, newBidExchangeName, newBidQueueName)).Methods("POST")
	myRouter.HandleFunc(fmt.Sprintf("/api/%s/cancelAuction/{itemId}", apiVersion), cancelAuction(auctionservice))
	myRouter.HandleFunc(fmt.Sprintf("/api/%s/stopAuction/{itemId}", apiVersion), stopAuction(auctionservice))
	myRouter.HandleFunc(fmt.Sprintf("/api/%s/ItemsUserHasBidsOn/{userId}", apiVersion), getItemsUserHasBidsOn(auctionservice)).Methods("GET")
	myRouter.HandleFunc(fmt.Sprintf("/api/%s/activeAuctions/", apiVersion), getActiveAuctions(auctionservice)).Methods("GET")
	// get active auctions

	// myRouter.HandleFunc("/publishNotifc", publishNotif)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

const (
	inMemoryFlag string = "inmemory"
	sqlFlag      string = "sql"
)

func getUsageStr() string {
	return "Usage: main DBTYPE\n" + fmt.Sprintf("    DBTYPE = one of ['%s','%s']; which database to use\n", inMemoryFlag, sqlFlag)
}

func fillReposWDummyData(bidRepo domain.BidRepository, auctionRepo domain.AuctionRepository) {
	// fill bid repo with some bids
	time1 := time.Date(2014, 2, 4, 00, 00, 00, 0, time.UTC)
	time2 := time.Date(2014, 2, 4, 00, 00, 00, 0, time.UTC)    // same as time1
	time3 := time.Date(2014, 2, 4, 00, 00, 00, 1000, time.UTC) // 1 microsecond after
	time4 := time.Date(2014, 2, 4, 00, 00, 01, 0, time.UTC)    // 1 sec after
	bid1 := *domain.NewBid("101", "20", "asclark109", time1, int64(300), true)
	bid2 := *domain.NewBid("102", "20", "mcostigan9", time2, int64(300), true)
	bid3 := *domain.NewBid("103", "20", "katharine2", time3, int64(400), true)
	bid4 := *domain.NewBid("104", "20", "katharine2", time4, int64(10), true)
	bidRepo.SaveBid(&bid1)
	bidRepo.SaveBid(&bid2)
	bidRepo.SaveBid(&bid3)
	bidRepo.SaveBid(&bid4)

	startime := time.Date(2014, 2, 4, 01, 00, 00, 0, time.UTC)
	endtime := time.Date(2014, 2, 4, 01, 30, 00, 0, time.UTC)                    // 30 min later
	item1 := domain.NewItem("101", "asclark109", startime, endtime, int64(2000)) // $20 start price
	item2 := domain.NewItem("102", "asclark109", startime, endtime, int64(2000)) // $20 start price
	auction1 := auctionRepo.NewAuction(item1, nil, nil, false, false, nil)       // will go to completion
	auction2 := auctionRepo.NewAuction(item2, nil, nil, false, false, nil)       // will get cancelled halfway through

	nowtime := time.Now()
	latertime := nowtime.Add(time.Duration(4) * time.Hour)                        // 4 hrs from now
	item3 := domain.NewItem("103", "asclark109", nowtime, latertime, int64(2000)) // $20 start price
	auctionactive := auctionRepo.NewAuction(item3, nil, nil, false, false, nil)

	latertime2 := nowtime.Add(time.Duration(2) * time.Hour)                        // 2 hrs from now
	item4 := domain.NewItem("104", "asclark109", nowtime, latertime2, int64(2000)) // $20 start price
	auctionactive2 := auctionRepo.NewAuction(item4, nil, nil, false, false, nil)

	auctionRepo.SaveAuction(auction1)
	auctionRepo.SaveAuction(auction2)
	auctionRepo.SaveAuction(auctionactive)
	auctionRepo.SaveAuction(auctionactive2)

}

func main() {

	// interpret args
	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) != 1 {
		fmt.Println("incorrect number of args provided")
		fmt.Println(getUsageStr())
		return
	}

	flagStr := argsWithoutProg[0]
	if !(flagStr == inMemoryFlag || flagStr == sqlFlag) { // use inMemory repository or use SQL repository?
		fmt.Println("unrecgonized arg provided: ", flagStr)
		fmt.Println(getUsageStr())
		return
	}

	// RABBITMQ PARAMS
	// parameters below need to be manually changed with deployment
	rabbitMQContainerHostName := "rabbitmq-server"
	rabbitMQContainerPort := "5672"

	startSoonExchangeName := "auction.start-soon"
	startSoonQueueName := ""

	endSoonExchangeName := "auction.end-soon"
	endSoonQueueName := ""

	auctionEndExchangeName := "auction.end"
	auctionEndQueueName := ""

	newBidExchangeName := "auction.new-bid"
	newBidQueueName := "auction.process-bid"

	ItemCounterfeitExchangeName := "item.counterfeit"
	ItemCounterfeitQueueName := "auction.consume_counterfeit"

	ItemInappropriateExchangeName := "item.innapropriate"
	ItemInappropriateQueueName := "auction.consume_innapropriate"

	UserUpdateExchangeName := "user.update"
	UserUpdateQueueName := "auction.consume_userupdate"

	UserDeleteExchangeName := "user.delete"
	UserDeleteQueueName := "auction.consume_userdelete"

	var conn *amqp.Connection

	// intialize OUTBOUND ADAPTER (AlertEngine)
	// AlertEngine holds methods for sending msgs outbound from the context;
	// Auctions hold a reference to a single AlertEngine, which they use to send msgs outbound (e.g. to RabbitMQ)
	useStubbedAlertEngine := false // MANUALLY CHANGE; if true, sends msgs to console instead; if false, sends msgs to RabbitMQ/Contexts

	var alertEngine domain.AlertEngine

	// typical to have 1 RabbitMQ connection per application
	connStr := fmt.Sprintf("amqp://guest:guest@%s:%s/", rabbitMQContainerHostName, rabbitMQContainerPort)
	var err error
	conn, err = amqp.Dial(connStr)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close() // close connection on application exit

	if useStubbedAlertEngine { // send outbound msgs to console
		fmt.Println("OUTBOUND ADAPTER = stubbed [outbound msgs go to console]...")
		alertEngine = domain.NewConsoleAlertEngine() // sends outbound msgs to console
		defer alertEngine.TurnDown()                 // nothing happens

	} else { // send outbound msgs to RabbitMQ and other contexts
		fmt.Println("OUTBOUND ADAPTER = deployment [outbound msgs go to RabbitMQ / other contexts]...")

		alertEngine = domain.NewRabbitMQAlertEngine( // creates a channel internally with connection
			conn,
			rabbitMQContainerHostName,
			startSoonExchangeName,
			startSoonQueueName,
			endSoonExchangeName,
			endSoonQueueName,
			auctionEndExchangeName,
			auctionEndQueueName,
		)
		defer alertEngine.TurnDown() // close channel(s) on application exit
	}

	// intialize REPOSITORIES
	var bidRepo domain.BidRepository
	var auctionRepo domain.AuctionRepository
	if flagStr == inMemoryFlag { // use in-memory repositories
		fmt.Println("REPOSITORY TYPE = in-memory [no persistence; everything held in-memory]")
		bidRepo = domain.NewInMemoryBidRepository(false) // do not use seed; assign random uuid's to new Bids
		auctionRepo = domain.NewInMemoryAuctionRepository(alertEngine)
		// fillReposWDummyData(bidRepo, auctionRepo) // seed with data?
	} else if flagStr == sqlFlag { // use SQL-based repositories
		fmt.Println("REPOSITORY TYPE = postgres SQL [data persisted to docker-volume on localhost]")
		bidRepo = domain.NewPostgresSQLBidRepository(false)                        // do not use seed; assign random uuid's to new Bids
		auctionRepo = domain.NewPostgresSQLAuctionRepository(bidRepo, alertEngine) // uses bidRepo to add references to Auction objs
	} else {
		fmt.Println("unrecgonized arg provided: ", flagStr)
		fmt.Println(getUsageStr())
		return
	}

	// seed with data?
	// if flagStr == inMemoryFlag {
	// 	fmt.Println("bid repository: ", bidRepo)
	// 	fillReposWDummyData(bidRepo, auctionRepo)
	// }

	log.Printf("Auctions Service API v1.0 - [Mux Routers impl for HTTP/RESTful API; RabbitMQ for messaging]")

	// initialize AUCTION-SERVICE
	auctionservice := NewAuctionService(bidRepo, auctionRepo)

	// initialize AUCTION-SESSION-MANAGER (description below):
	// spawns goroutines that will invoke auctionservice periodically to do internal house-keeping;
	// AuctionSessionManager.TurnOn() spawns 3 goroutines that each are responsible for periodically invoking
	// the auctionservice to send out alerts, finalize (archive) auctions that are over, and load into memory
	// auctions that start soon. these goroutines return (stop) when an internal bool variable of AuctionSessionManager
	// becomes false (TurnOn() sets the variable to true and spawns the goroutines; the goroutines only exit
	// when the boolean variable goes to false; TurnOff() sets this variable to false). This implementation works
	// as-written but is not strictly correct. It works because the application calls TurnOn() only when application
	// starts and never calls TurnOff(). However, as-written it is not correct to intermittently call TurnOn(), TurnOff(),
	// TurnOn(), etc. A race condition may occur with a sequence of 3 calls: TurnOn(), TurnOff(), TurnOn(). It is
	// possible by race condition that this will not cause the first 3 goroutines to exit and will spawn an additional
	// 3 goroutines, leading to a total of 6. This can be refactored to be correct by passing the spawned goroutines a
	// bool channel, which they will use to determine when they should exit.
	alertCycle := time.Duration(10) * time.Second       // how often the service should introspect to see if alerts should be sent out
	finalizeCycle := time.Duration(6) * time.Second     // how often the service should introspect to see if auctions should be finalized
	loadAuctionCycle := time.Duration(10) * time.Second // how often the service should introspect to see if it should load new auctions into memory
	auctionSessionManager := NewAuctionSessionManager(auctionservice, alertCycle, finalizeCycle, loadAuctionCycle)
	auctionSessionManager.TurnOn() // starts the periodic invocation of auctionservice

	// spawn goroutines that will invoke auctionservice upon incoming HTTP/RESTful requests
	go handleHTTPAPIRequests(auctionservice, conn, newBidExchangeName, newBidQueueName)

	// spawn goroutines that will invoke auctionservice upon incoming RabbitMQ messages

	go handleNewBids(auctionservice, conn, newBidExchangeName, newBidQueueName)
	go handleItemCounterfeit(auctionservice, conn, ItemCounterfeitExchangeName, ItemCounterfeitQueueName)
	go handleItemInappropriate(auctionservice, conn, ItemInappropriateExchangeName, ItemInappropriateQueueName)
	go handleUserDelete(auctionservice, conn, UserUpdateExchangeName, UserUpdateQueueName)
	go handleUserUpdate(auctionservice, conn, UserDeleteExchangeName, UserDeleteQueueName)

	// time1 := time.Date(2014, 2, 4, 00, 00, 00, 0, time.UTC)
	// time2 := time.Date(2014, 2, 4, 00, 00, 00, 0, time.UTC)    // same as time1
	// time3 := time.Date(2014, 2, 4, 00, 00, 00, 1000, time.UTC) // 1 microsecond after
	// time4 := time.Date(2014, 2, 4, 00, 00, 01, 0, time.UTC)    // 1 sec after
	// bid1 := *domain.NewBid("101", "20", "asclark109", time1, int64(300), true)
	// bid2 := *domain.NewBid("102", "20", "mcostigan9", time2, int64(300), true)
	// bid3 := *domain.NewBid("103", "20", "katharine2", time3, int64(400), true)
	// bid4 := *domain.NewBid("104", "20", "katharine2", time4, int64(10), true)
	// bidRepo.SaveBid(&bid1)
	// bidRepo.SaveBid(&bid2)
	// bidRepo.SaveBid(&bid3)
	// bidRepo.SaveBid(&bid4)
	// bids := []*domain.Bid{&bid1, &bid2, &bid3, &bid4}
	// bids := []*domain.Bid{}

	// startime := time.Now()
	// endtime := startime.Add(time.Duration(10) * time.Minute)
	// item1 := domain.NewItem("20", "asclark109", startime, endtime, int64(2000)) // $20 start price
	// auction1 := auctionRepo.NewAuction(item1, &bids, nil, false, false, nil)    // will go to completion
	// // auctionRepo.SaveAuction(auction1)

	// time.Sleep(time.Duration(5) * time.Second)

	// fmt.Println("canceling new auction")
	// auctionservice.StopAuction(item1.ItemId)

	// lastTime := time.Now()
	// for {

	// 	if time.Since(lastTime) >= time.Duration(10)*time.Second {
	// 		fmt.Println(runtime.NumGoroutine())
	// 		lastTime = time.Now()
	// 	}
	// }

	var forever chan struct{}
	<-forever
}

type CustomerData struct {
	Customer_id string
	Store_id    int64
	First_name  string
	Last_name   string
	Email       string
	Address_id  string
	Activebool  string
	Create_date string
	Last_update string
	Active      string
}
