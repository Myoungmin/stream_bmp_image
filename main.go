package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/image/bmp"
)

var (
	fps       = 60.0
	interval  = int(1000000 / fps)
	width     = 1024
	height    = 1024
	frame     = 0
	sentFrame = 0
	// [5][]byte{createImage(0), createImage(1), createImage(2), createImage(3), createImage(4)} 배열 리터럴 생성
	// 이를 참조하는 슬라이스 images
	images     = [][]byte{createImage(0), createImage(1), createImage(2), createImage(3), createImage(4)}
	openSocket = false
	start      time.Time
)

func main() {
	port := 8080

	http.HandleFunc("/", socketHandler)

	fmt.Printf("Listening on port %d...\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

// WebSocket 업그레이드 과정은 클라이언트가 HTTP 연결을 WebSocket 프로토콜로 업그레이드하도록 요청
// ReadBufferSize 및 WriteBufferSize 필드는 들어오고 나가는 데이터의 버퍼 크기를 관리하여 WebSocket 통신 중 성능과 메모리 사용을 최적화하는 역할
var upgrader = websocket.Upgrader{
	ReadBufferSize:  0,
	WriteBufferSize: 0,
}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	// HTTP 연결을 WebSocket 프로토콜로 업그레이드
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade error: %s\n", err.Error())
		return
	}
	defer conn.Close()

	fmt.Println("WebSocket opened.")

	// Channel은 채널 연산자인 <- 을 통해 값을 주고 받을 수 있는 하나의 분리된 통로
	// Channel은 map과 slice처럼 사용하기 전에 생성
	// 전송과 수신은 다른 한 쪽이 준비될 때까지 block 상태
	// 명시적인 lock이나 조건 변수 없이 goroutine이 synchronous하게 작업될 수 있도록한다.
	eventCh := make(chan string)

	// goroutine 은 Go 런타임에 의해 관리되는 경량 쓰레드
	// 이벤트를 비동기적으로 처리하기 위해 goroutine 실행, 익명함수를 사용한 goroutine
	// 웹 소켓으로부터 수신된 이벤트를 읽어서 eventCh 채널을 통해 메인 goroutine으로 전송
	go func() {
		for {
			_, event, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("WebSocket error: %s\n", err.Error())
				return
			}
			// 채널 eventCh에 이벤트를 전송
			eventCh <- string(event)
		}
	}()

	// time으로 보내야할 frame 계산하는 goroutine
	go func() {
		for {
			frame = int(time.Since(start).Microseconds()) / interval
		}
	}()

	// 보내야할 frame보다 보낸 frame이 적으면 send하는 goroutine
	go func() {
		for {
			if sentFrame < frame {
				sentFrame++
				err := conn.WriteMessage(websocket.BinaryMessage, images[sentFrame%5])
				if err != nil {
					fmt.Printf("Error writing frame: %s\n", err.Error())
					return
				}
				fmt.Println("Frame: ", sentFrame)
			}
		}
	}()

	for {
		// select: goroutine이 다중 커뮤니케이션 연산에서 대기할 수 있게 한다.
		// case들 중 하나가 실행될 때까지 block
		// 다수의 case가 준비되는 경우에는 select가 무작위로 하나를 선택
		select {
		// goroutine에서 비동기적으로 이벤트를 수신하였을 때
		case event := <-eventCh:
			// 이미지 크기 수신 시 설정하여 이미지 재생성
			if strings.Contains(event, ",") {
				options := strings.Split(event, ",")
				width, _ = strconv.Atoi(options[0])
				height, _ = strconv.Atoi(options[1])
				fps, _ = strconv.ParseFloat(options[2], 32)
				fmt.Println("Client Width: ", width, "Height: ", height, "FPS: ", fps)

				images = [][]byte{createImage(0), createImage(1), createImage(2), createImage(3), createImage(4)}
				interval = int(1000000 / fps)
			} else if event == "start" {
				openSocket = true
				start = time.Now()
				sentFrame = 0
			} else if event == "quit" {
				openSocket = false
				sentFrame = 0
			}
		}
	}
}

func createImage(i int) []byte {
	img := image.NewGray(image.Rect(0, 0, width, height))

	// createImage 호출 시 랜덤 픽셀 이미지 생성
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			randNum := uint8(rand.Intn(256))
			c := color.Gray{randNum}
			img.Set(x, y, c)
		}
	}

	buf := new(bytes.Buffer)
	if err := bmp.Encode(buf, img); err != nil {
		log.Fatalf("failed to encode: %v", err)
	}
	fmt.Println("size: ", len(buf.Bytes()))

	return buf.Bytes()
}
