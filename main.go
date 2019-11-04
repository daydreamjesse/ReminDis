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

var Token string
var mongoDBP string
var user string
var cmdMap map[string]string
var cmdSlice []string
var shards []string
var coll *mongo.Collection

type TaskFields struct {
	task string `json:"task"`
	priority string `json:"priority"`
	uid string `json:"uid"`
}

func init() {
	fmt.Println("Initializing...")
	readFile()
	shards = append(shards, "remindis-v1-shard-00-00-pfykt.mongodb.net:27017", "remindis-v1-shard-00-01-pfykt.mongodb.net:27017", "remindis-v1-shard-00-02-pfykt.mongodb.net:27017")
	cmdMap = make(map[string]string)
	mongoConn()
}

func main() {

	dg, err := discordgo.New("Bot " + Token)
	fmt.Println("Establishing Discord session with token... ")

	if err != nil {
		fmt.Println("Failed to establish Discord session: ", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	fmt.Println("Opening websocket connection to Discord...")
	if err != nil {
		fmt.Println("Error opening connection to Discord: ", err)
		fmt.Println("Token used: " + Token)
		return
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {

	// if ReminDis is the one sending the message, ignore
	if msg.Author.ID == s.State.User.ID {
		return
	}

	// if a message contains "!remindis", parse the command
	if (strings.Contains(msg.Content, "!remindis")) {
		s.ChannelMessageSend(msg.ChannelID, "Message Received!")
		user = msg.Author.ID
		parseCommand(msg.Content)
	}
}

func parseCommand(cmd string) {
	cmd = strings.ReplaceAll(cmd, "!remindis", "")
	var cmdArr = strings.Split(cmd, "{")
	for i := 0; i < len(cmdArr); i++ {
		if i != (len(cmdArr) - 1) {
			if (strings.Contains(cmdArr[i], "}")) {
				var split = strings.Split(cmdArr[i], "}")
				cmdSlice = append(cmdSlice, split[0], split[1])
			} else {
				cmdSlice = append(cmdSlice, cmdArr[i])
			}
		} else {
			cmdSlice = append(cmdSlice, strings.ReplaceAll(cmdArr[i], "}", ""))
		}
	}
	//fmt.Println(cmdSlice, len(cmdSlice))

	cmdLength := len(cmdSlice)
	var commands []string
	var parameters []string
	for i := 0; i < cmdLength; i++ {
		if (i%2==0) {
			commands = append(commands, cmdSlice[i])
		} else {
			parameters = append(parameters, cmdSlice[i])
		}
	}
	for i := 0; i < len(commands); i++ {
		//fmt.Printf("cmdMap[%s] = %s\n", commands[i], parameters[i])
		cmdMap[commands[i]] = parameters[i]
	}
	cmdMap["user"] = user
	handleCommands()
}

func handleCommands() {
	//fmt.Printf("task: %s\npriority: %s\nuser: %s\n", cmdMap["task"], cmdMap["priority"], cmdMap["user"])
	taskToStore := TaskFields {
		task: cmdMap[" task"],
		priority: cmdMap[" priority"],
		uid: cmdMap["user"],
	}
	mongoInsert(taskToStore)
	purgeMap()
}

func purgeMap() {
	for key := range cmdMap {
		delete(cmdMap, key)
	}
	for i := (len(cmdSlice) - 1); i >= 0; i-- {
		cmdSlice = remove(cmdSlice, i)
	}
}

func remove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func readFile() {
	file, err := os.Open("credentials.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	data := fmt.Sprintf("%s", b)
	newData := strings.Split(data, "\n")
	Token = newData[0]
	mongoDBP = newData[1]
}

func mongoConn() {

	//fmt.Printf(("mongodb:/"+"/remindis_discord:%s@%s,%s,%s/admin?ssl=true&replicaSet=remindis-v1-shard-0&authSource=admin&retryWrites=true&w=majority"), mongoDBP, shards[0], shards[1], shards[2])
	fmt.Println("Connecting to database...")
	clientOptions := options.Client().ApplyURI(
		fmt.Sprintf(("mongodb:/"+"/remindis_discord:%s@%s,%s,%s/admin?ssl=true&replicaSet=remindis-v1-shard-0&authSource=admin&retryWrites=true&w=majority"), mongoDBP, shards[0], shards[1], shards[2]),
	)

	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil{
		fmt.Println("Connect Error:", err)
		os.Exit(1)
	}

	coll = client.Database("alpha-v0_1").Collection("testing")

}

func mongoInsert(task TaskFields) {
	t1 := time.Now()
	_, insertErr := coll.InsertOne(
		context.TODO(),
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
