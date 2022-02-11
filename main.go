package main

import (
	"flag"
	"go-ws/core"
	"log"
	"net/http"
)

func main() {
	port := flag.String("p", ":8080", "port")
	dir := flag.String("d", ".", "dir")
	flag.Parse()

	server := gows.NewServer()

	go func() {
		for {
			socket, err := server.Accept()
			if err != nil {
				log.Fatalln("accept error:", err)
			}

			go func() {
				defer socket.Close()

				for {
					if frame, err := socket.Recv(); err != nil {
						log.Fatalln("recv error:", err)
						break
					} else {
						log.Println(string(frame.Payload))
					}

				}

			}()

		}
	}()

	http.Handle("/statics/", http.StripPrefix("/statics/", http.FileServer(http.Dir(*dir))))
	http.Handle("/ws", server)

	log.Println("server listen at ", *port)

	log.Fatal(http.ListenAndServe(*port, nil))

}
