package main

import (
	"github.com/joho/godotenv"
	"github.com/qcodelabsllc/qreeket/groups/config"
	"github.com/qcodelabsllc/qreeket/groups/network"
	"log"
	"os"
)

func main() {
	// This line loads the environment variables from the ".env" file.
	if err := godotenv.Load(); err != nil {
		log.Fatalf("unable to load environment variables: %+v\n", err)
	}

	// This line initializes the crash logs
	config.InitCrashLogs()

	// This line initializes the database connection
	if client, err := config.InitDatabaseConnection(); err != nil {
		panic(err)
	} else {
		// get database name from environment variables
		dbName := os.Getenv("DATABASE_NAME")

		// get collection names from environment variables (groups & channels)
		groupsCollectionName := os.Getenv("GROUPS_COLLECTION")
		channelsCollectionName := os.Getenv("CHANNELS_COLLECTION")
		invitesCollectionName := os.Getenv("INVITES_COLLECTION")
		subscriptionRequestsCollectionName := os.Getenv("SUBSCRIPTION_REQUESTS_COLLECTION")

		// This line initializes the groups collection
		groupsCollection := client.Database(dbName).Collection(groupsCollectionName)

		// This line initializes the channels collection
		channelsCollection := client.Database(dbName).Collection(channelsCollectionName)

		// This line initializes the invites collection
		invitesCollection := client.Database(dbName).Collection(invitesCollectionName)

		// This line initializes the subscriptions collection
		subscriptionsCollection := client.Database(dbName).Collection(subscriptionRequestsCollectionName)

		// This line initializes the grpc server
		network.InitServer(groupsCollection, channelsCollection, invitesCollection, subscriptionsCollection)
	}

}
