package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {

	dir, port, err := args()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(dir, port)

	// Create fileserver for dir
	fs := http.FileServer(http.Dir(dir))

	http.Handle("/", fs)

	// run file server
	localIp, _ := getLocalIP()
	log.Println("\nFILE SERVER RUN \nSHARED DIR: ", dir, "\nLOCAL ADDR: ", localIp, ":", port)
	port = ":" + port
	err = http.ListenAndServe(port, nil)

	if err != nil {
		log.Fatal(err)
	}
}

func getLocalIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				return nil, err
			}

			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					if v.IP.To4() != nil {
						return v.IP, nil
					}
				case *net.IPAddr:
					if v.IP.To4() != nil {
						return v.IP, nil
					}
				}
			}
		}
	}

	return nil, nil
}

func args() (string, string, error) {

	var argDir string
	var argPort string
	var err error

	argRun := os.Args[1:]

	if len(argRun) < 2 {
		log.Fatalln("\nEmpty args.\n--dir [path to dir] - path to shared directory.\n--port [port] - port for file server.")
		err = errors.New("Empеy run arguments. Abort")
		return "", "", err
	}

	if argRun[0] == "--dir" {
		argDir = argRun[1]
		// return argDir, nil
	} else {
		err = errors.New("Unknown arg")
		// return "", err
	}

	if len(argRun) == 4 {
		if argRun[2] == "--port" {
			argPort = argRun[3]
		} else {
			err = errors.New("Unknown arg")
			argPort = "8080"
		}
	} else {
		argPort = "8080"
	}

	return argDir, argPort, err
}
