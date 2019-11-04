package main

import (
	"fmt"
	"os"
	"os/signal"
	"io/ioutil"
	"log"
	"syscall"
	"context"
	"time"
	//"reflect"
	"strings"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson"
)

// Global variables
var Token string
var mongoDBP string
var user string
var cmdMap map[string]string
var cmdSlice []string
var shards []string
var coll *mongo.Collection

// structure to hold information until we insert to db
type TaskFields struct {
	task string `json:"task"`
	priority string `json:"priority"`
	uid string `json:"uid"`
}

// initialize by getting credentials, setting shard strings, and making map
func init() {
	fmt.Println("Initializing...")
	readFile()
	shards = append(shards, "remindis-v1-shard-00-00-pfykt.mongodb.net:27017", "remindis-v1-shard-00-01-pfykt.mongodb.net:27017", "remindis-v1-shard-00-02-pfykt.mongodb.net:27017")
	cmdMap = make(map[string]string)
	mongoConn()
}

// main logic loop
func main() {

	// establish connection to Discord
	dg, err := discordgo.New("Bot " + Token)
	fmt.Println("Establishing Discord session with token... ")

	if err != nil {
		fmt.Println("Failed to establish Discord session: ", err)
		return
	}

	// create a handler for new messages
	dg.AddHandler(messageCreate)

	// open a websocket connection to discord
	err = dg.Open()
	fmt.Println("Opening websocket connection to Discord...")
	if err != nil {
		// If there is an error, print out the token to compare to api
		fmt.Println("Error opening connection to Discord: ", err)
		fmt.Println("Token used: " + Token)
		return
	}

	// Bot is now up and running
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

// Message handler for when someone sends a message
func messageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {

	// if ReminDis is the one sending the message, ignore
	if msg.Author.ID == s.State.User.ID {
		return
	}

	// if a message contains "!remindis", parse the command
	if (strings.Contains(msg.Content, "!remindis")) {
		// Let them know we've received it
		s.ChannelMessageSend(msg.ChannelID, "Message Received!")
		// set user id to whoever sent the message
		user = msg.Author.ID
		// parse the command
		parseCommand(msg.Content)
	}
}

// Takes in the entire message that contains !remindis
func parseCommand(cmd string) {
	// We don't need !remindis anymore, so get rid of it
	cmd = strings.ReplaceAll(cmd, "!remindis", "")
	// Slice through the string and split up the commands
	var cmdArr = strings.Split(cmd, "{")
	for i := 0; i < len(cmdArr); i++ {
		if i != (len(cmdArr) - 1) {
			// Since we already split by { we need to recognize the next command by splitting with }
			if (strings.Contains(cmdArr[i], "}")) {
				var split = strings.Split(cmdArr[i], "}")
				cmdSlice = append(cmdSlice, split[0], split[1])
			// If it doesn't need to be split, go ahead and append it
			} else {
				cmdSlice = append(cmdSlice, cmdArr[i])
			}
		// If it contains } but is the last one, there are no more commands to be parse, so just remove the }
		} else {
			cmdSlice = append(cmdSlice, strings.ReplaceAll(cmdArr[i], "}", ""))
		}
	}
	//fmt.Println(cmdSlice, len(cmdSlice))

	// get loop iteration length
	cmdLength := len(cmdSlice)
	// declare two slices, one for commands and one for parameters
	var commands []string
	var parameters []string
	// go through the main cmd slice and pull out the appropriate strings
	for i := 0; i < cmdLength; i++ {
		if (i%2==0) {
			commands = append(commands, cmdSlice[i])
		} else {
			parameters = append(parameters, cmdSlice[i])
		}
	}
	// take the slices we just made and map them
	for i := 0; i < len(commands); i++ {
		//fmt.Printf("cmdMap[%s] = %s\n", commands[i], parameters[i])
		cmdMap[commands[i]] = parameters[i]
	}
	// also need to map the user info we got from earlier
	cmdMap["user"] = user
	// now that we've parsed the command, we can handle the command
	handleCommands()
}

// sets up a struct with necessary info
func handleCommands() {
	//fmt.Printf("task: %s\npriority: %s\nuser: %s\n", cmdMap["task"], cmdMap["priority"], cmdMap["user"])
	taskToStore := TaskFields {
		task: cmdMap[" task"],
		priority: cmdMap[" priority"],
		uid: cmdMap["user"],
	}
	// sends the struct to insert function
	mongoInsert(taskToStore)
	// purges the map for next user info
	purgeMap()
}

// clears out map and command slice
func purgeMap() {
	for key := range cmdMap {
		delete(cmdMap, key)
	}
	for i := (len(cmdSlice) - 1); i >= 0; i-- {
		cmdSlice = remove(cmdSlice, i)
	}
}

// a remove function for slices because slices are just arrays, so you gotta make new arrays without the item you don't want
func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// read the credentials file(omitted through .gitignore for obvious reasons)
func readFile() {
	// open the file
	file, err := os.Open("credentials.txt")
	if err != nil {
		log.Fatal(err)
	}
	// once we get the data we need, close the file
	defer file.Close()

	// read all of the bytes of the file
	b, err := ioutil.ReadAll(file)
	// format the bytes to strings
	data := fmt.Sprintf("%s", b)
	// split the strings by newline
	newData := strings.Split(data, "\n")
	// first line is bot token, second is password for mongodb atlas
	Token = newData[0]
	mongoDBP = newData[1]
}

// set up a connection to mdb, runs during init()
func mongoConn() {

	//fmt.Printf(("mongodb:/"+"/remindis_discord:%s@%s,%s,%s/admin?ssl=true&replicaSet=remindis-v1-shard-0&authSource=admin&retryWrites=true&w=majority"), mongoDBP, shards[0], shards[1], shards[2])
	fmt.Println("Connecting to database...")
	// set up the client options with the annoyingly long uri
	clientOptions := options.Client().ApplyURI(
		fmt.Sprintf(("mongodb:/"+"/remindis_discord:%s@%s,%s,%s/admin?ssl=true&replicaSet=remindis-v1-shard-0&authSource=admin&retryWrites=true&w=majority"), mongoDBP, shards[0], shards[1], shards[2]),
	)

	// establish connection with client
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil{
		fmt.Println("Connect Error:", err)
		os.Exit(1)
	}

	// set the collection to our alpha testing collection
	coll = client.Database("alpha-v0_1").Collection("testing")

}

// handles inserting to atlas db
func mongoInsert(task TaskFields) {
	// get the time for logging
	t1 := time.Now()
	_, insertErr := coll.InsertOne(
		context.TODO(),
		// take the struct we get and turn it into a bson object(mdb's preferred data structure)
		bson.D{
			{"task", task.task},
			{"priority", task.priority},
			{"uid", task.uid},
		},
	)
	fmt.Println("Inserting doc took", time.Now().Sub(t1))

	if insertErr != nil {
		fmt.Println("Insert error:", insertErr)
		os.Exit(1)
	}
}
