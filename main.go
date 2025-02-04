package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	sdk "github.com/datasance/iofog-go-sdk/v3/pkg/microservices"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type LogLevel int

const (
	INFO LogLevel = iota
	WARN
	SEVERE
)

var currentLogLevel LogLevel

func initLogger() {
	logLevel := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch logLevel {
	case "INFO":
		currentLogLevel = INFO
	case "WARN":
		currentLogLevel = WARN
	case "SEVERE":
		currentLogLevel = SEVERE
	default:
		currentLogLevel = INFO
	}
}

func logMessage(level LogLevel, format string, v ...interface{}) {
	if level >= currentLogLevel {
		log.Printf(format, v...)
	}
}

type Subscriber struct {
	Config *Config
	mu     sync.Mutex
}

type Config struct {
	Topics   []Topic `json:"topics"`
	Token    string  `json:"token"`
	MqttHost string  `json:"mqttHost"`
	MqttPort int     `json:"mqttPort"`
}

type Topic struct {
	MainTopic  string   `json:"maintopic"`
	SubTopics  []string `json:"subtopics"`
	InfoType   string   `json:"infoType"`
	InfoFormat string   `json:"infoFormat"`
}

var mqttSubscriber *Subscriber

func init() {
	initLogger()
	mqttSubscriber = new(Subscriber)
	mqttSubscriber.Config = new(Config)
}

func main() {
	ioFogClient, clientError := sdk.NewDefaultIoFogClient()
	if clientError != nil {
		logMessage(SEVERE, "Fatal error: %s", clientError.Error())
		os.Exit(1)
	}

	if err := updateConfig(ioFogClient, mqttSubscriber.Config); err != nil {
		logMessage(SEVERE, "Fatal error: %s", err.Error())
		os.Exit(1)
	}

	confChannel := ioFogClient.EstablishControlWsConnection(0)
	exitChannel := make(chan error)
	go createSubscriber(ioFogClient)

	for {
		select {
		case <-exitChannel:
			os.Exit(0)
		case <-confChannel:
			newConfig := new(Config)
			if err := updateConfig(ioFogClient, newConfig); err != nil {
				logMessage(SEVERE, "Error updating config: %s", err)
			} else {
				updateSubscriber(ioFogClient, newConfig)
			}
		}
	}
}

func createSubscriber(ioFogClient *sdk.IoFogClient) {
	dataChannel, receiptChannel := ioFogClient.EstablishMessageWsConnection(0, 0)
	if dataChannel == nil || receiptChannel == nil {
		logMessage(WARN, "Failed to establish WebSocket connection.")
		return
	}
	logMessage(INFO, "Message WebSocket connection established.")

	go func() {
		for {
			select {
			case iomsg := <-dataChannel:
				logMessage(INFO, "Received IoMessage: %v", iomsg)
			case r := <-receiptChannel:
				logMessage(INFO, "Received receipt response: %v", r)
			}
		}
	}()

	broker := fmt.Sprintf("tcp://%s:%d", mqttSubscriber.Config.MqttHost, mqttSubscriber.Config.MqttPort)
	opts := mqtt.NewClientOptions().AddBroker(broker)
	opts.SetClientID("mqtt-iomessage")
	opts.SetUsername(mqttSubscriber.Config.Token)
	opts.SetPassword("")
	opts.SetDefaultPublishHandler(messageHandler(ioFogClient))
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logMessage(SEVERE, "Failed to connect to MQTT broker: %v", token.Error())
		return
	}
	logMessage(INFO, "Connected to MQTT broker at %s", broker)

	for _, topic := range mqttSubscriber.Config.Topics {
		subscribeTopic(client, topic)
	}
}

func subscribeTopic(client mqtt.Client, topic Topic) {
	for _, sub := range topic.SubTopics {
		fullTopic := fmt.Sprintf("%s/%s/#", topic.MainTopic, sub)
		token := client.Subscribe(fullTopic, 1, nil)
		token.Wait()
		if token.Error() != nil {
			logMessage(WARN, "Failed to subscribe to topic %s: %v", fullTopic, token.Error())
		} else {
			logMessage(INFO, "Subscribed to topic: %s", fullTopic)
		}
	}
}

func messageHandler(ioFogClient *sdk.IoFogClient) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		topicParts := splitTopic(msg.Topic())
		if len(topicParts) < 3 {
			logMessage(WARN, "Invalid topic format: %s", msg.Topic())
			return
		}
		mainTopic := topicParts[0]
		subTopic := topicParts[1]
		deviceID := strings.Join(topicParts[2:], "/")

		var infoType, infoFormat string
		for _, topic := range mqttSubscriber.Config.Topics {
			if topic.MainTopic == mainTopic {
				infoType = topic.InfoType
				infoFormat = topic.InfoFormat
				break
			}
		}
		ioMessage := &sdk.IoMessage{
			InfoFormat:  infoFormat,
			InfoType:    infoType,
			AuthGroup:   mainTopic,
			AuthID:      fmt.Sprintf("%s/%s", subTopic, deviceID),
			ContentData: msg.Payload(),
		}
		if err := ioFogClient.SendMessageViaSocket(ioMessage); err != nil {
			logMessage(WARN, "Error sending IoMessage: %v", err)
		}
	}
}

func splitTopic(topic string) []string {
	return strings.Split(topic, "/")
}

func updateSubscriber(ioFogClient *sdk.IoFogClient, config *Config) {
	mqttSubscriber.mu.Lock()
	defer mqttSubscriber.mu.Unlock()
	mqttSubscriber.Config = config
	logMessage(INFO, "MQTT subscriber configuration updated successfully.")
}

func updateConfig(ioFogClient *sdk.IoFogClient, config interface{}) error {
	attemptLimit := 5
	var err error
	for err = ioFogClient.GetConfigIntoStruct(config); err != nil && attemptLimit > 0; attemptLimit-- {
		return err
	}
	if attemptLimit == 0 {
		return fmt.Errorf("Update config failed")
	}
	return nil
}
