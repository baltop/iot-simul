package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jinzhu/configor"
)

type DevNode struct {
	Topic    string  `required:"true" yaml:"topic"`
	Dev      string  `required:"true" yaml:"dev"`
	Tag      string  `required:"true" yaml:"tag"`
	Max      float32 `required:"false" yaml:"max"`
	Min      float32 `required:"false" yaml:"min"`
	Interval int     `required:"true" yaml:"interval"`
}

type baseConfig struct {
	APPName string `default:"app name"`

	Server string `default:"tcp://192.168.0.103:1883"`

	Mqtt []DevNode
}

var config = baseConfig{}

// main function.
func main() {

	var c_allCount chan int
	var ctx context.Context
	var cancel context.CancelFunc

	error := configor.New(&configor.Config{
		AutoReload:         true,
		AutoReloadInterval: 15 * time.Second,
		AutoReloadCallback: func(config interface{}) {
			fmt.Printf("====================================\n%v changed \n", config)
			fmt.Printf("====================================\n")
			ss := config.(*baseConfig)
			cancel()
			c_allCount = make(chan int)
			ctx, cancel = context.WithCancel(context.Background())
			start(ctx, *ss, c_allCount)

		},
	}).Load(&config, "config.yaml")

	if error != nil {
		log.Fatal(error)
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)

	c_allCount = make(chan int)

	ctx, cancel = context.WithCancel(context.Background())

	start(ctx, config, c_allCount)

	cc := 0

	for {
		select {
		case ind := <-c_allCount:
			cc += ind
		case <-sigs:
			cancel()
			fmt.Println(time.Now(), "--", cc)
			return
		}
	}

}

func start(ctx context.Context, config baseConfig, c_allCount chan<- int) {

	fmt.Println("start startfunc")

	opts := mqtt.NewClientOptions().AddBroker(config.Server)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
	}
	for _, devNode := range config.Mqtt {
		go handleMeasure(ctx, client, devNode, c_allCount)
	}
}

func handleMeasure(ctx context.Context, client mqtt.Client, devNode DevNode, c_allCount chan<- int) {

	fmt.Println("start handleMeasure", devNode.Tag)

	type reqMessage struct {
		To  string `json:"to"`
		Fr  string `json:"fr"`
		Rqi string `json:"rqi"`
		Pc  struct {
			Cin struct {
				Con float32 `json:"con"`
			} `json:"m2m:cin"`
		} `json:"pc"`
		Op  int `json:"op"`
		Ty  int `json:"ty"`
		Sec int `json:"sec"`
	}

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	ramdonSerial := r1.Intn(100000000)

	for {
		ramdonSerial++
		rqiStr := fmt.Sprintf("472c-9d83-e22bx23-%d", ramdonSerial)

		reqMessageValue := &reqMessage{
			To:  devNode.Dev + "/" + devNode.Tag,
			Fr:  "SiotTestAE",
			Rqi: rqiStr,
			Op:  1,
			Ty:  4,
			Sec: 0,
		}
		reqMessageValue.Pc.Cin.Con = randFloat(devNode.Min, devNode.Max)

		brt, err := json.Marshal(reqMessageValue)
		if err != nil {
			fmt.Println(err)
		}

		// simValue := randFloat(devNode.Min, devNode.Max)
		// cmMessage := "{\"to\":\"" + devNode.Dev + "/" + devNode.Tag + "\",\"fr\":\"SiotTestAE\",\"rqi\":\"" + rqiStr + "\",\"pc\":{\"m2m:cin\":{\"con\":\"" + simValue + "\"}},\"op\":1,\"ty\":4,\"sec\":0}"
		// brt := []byte(cmMessage)

		if token := client.Publish(devNode.Topic, 1, false, brt); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
		}

		c_allCount <- 1

		select {
		case <-ctx.Done(): // if cancel() execute
			fmt.Println("canceled by context", devNode.Tag, devNode.Interval)
			return
		default:
			fmt.Println("for loop", devNode.Tag, devNode.Interval)
		}

		time.Sleep(time.Duration(devNode.Interval) * time.Millisecond)

	}
}

func randFloat(min float32, max float32) float32 {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	return min + r1.Float32()*(max-min)
}
