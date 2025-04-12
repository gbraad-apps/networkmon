package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func getNetworkStats(device string) (int64, int64, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, device) {
			fields := strings.Fields(line)
			rx, _ := strconv.ParseInt(fields[1], 10, 64) // Received bytes
			tx, _ := strconv.ParseInt(fields[9], 10, 64) // Transmitted bytes
			return rx, tx, nil
		}
	}
	return 0, 0, fmt.Errorf("device not found")
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
	<title>Network Throughput</title>
	<style>
		html, body, canvas {
			margin: 0;
			padding: 0;
			width: 100%;
			height: 100%;
			overflow: hidden;
		}
		canvas {
			display: block;
		}
	</style>
</head>
<body>
	<canvas id="graph"></canvas>
	<script>
		let params = new URLSearchParams(window.location.search);
		let device = params.get("device") || "eth0";
		let socket = new WebSocket("ws://" + location.host + "/ws?device=" + device);
		let canvas = document.getElementById("graph");
		let ctx = canvas.getContext("2d");

		canvas.width = window.innerWidth;
		canvas.height = window.innerHeight;

		let rxData = [], txData = [];
		let maxPoints = 100;

		socket.onmessage = function(event) {
			let data = JSON.parse(event.data);
			rxData.push(data.rx);
			txData.push(data.tx);

			if (rxData.length > maxPoints) rxData.shift();
			if (txData.length > maxPoints) txData.shift();

			// Draw graph
			ctx.clearRect(0, 0, canvas.width, canvas.height);

			// Draw RX (blue)
			ctx.beginPath();
			ctx.strokeStyle = "blue";
			rxData.forEach((val, i) => {
				let x = (i / maxPoints) * canvas.width;
				let y = canvas.height - (val / 1000); // Scale factor
				if (i === 0) ctx.moveTo(x, y);
				else ctx.lineTo(x, y);
			});
			ctx.stroke();

			// Draw TX (green)
			ctx.beginPath();
			ctx.strokeStyle = "green";
			txData.forEach((val, i) => {
				let x = (i / maxPoints) * canvas.width;
				let y = canvas.height - (val / 1000); // Scale factor
				if (i === 0) ctx.moveTo(x, y);
				else ctx.lineTo(x, y);
			});
			ctx.stroke();
		};
	</script>
</body>
</html>
`
	w.Write([]byte(html))
}

func serveWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade:", err)
		return
	}
	defer conn.Close()

	// Get the network device from the query parameters
	query := r.URL.Query()
	device := query.Get("device")
	if device == "" {
		device = "eth0" // Default to eth0
	}

	var prevRx, prevTx int64

	for {
		rx, tx, err := getNetworkStats(device)
		if err != nil {
			log.Println("Error reading network stats:", err)
			return
		}

		rxRate := rx - prevRx
		txRate := tx - prevTx
		prevRx, prevTx = rx, tx

		data := map[string]int64{"rx": rxRate, "tx": txRate}
		if err := conn.WriteJSON(data); err != nil {
			log.Println("WebSocket write:", err)
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {
	http.HandleFunc("/", serveHTML)
	http.HandleFunc("/ws", serveWebSocket)

	log.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
