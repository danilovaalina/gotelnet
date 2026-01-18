package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	Host    string
	Port    int
	Timeout int
}

func parseArgs() (*Config, error) {
	var timeout int
	flag.IntVar(&timeout, "timeout", 10, "connection timeout in seconds")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <host> <port>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		return nil, fmt.Errorf("expected exactly 2 positional arguments: <host> <port>")
	}

	host := args[0]
	portStr := args[1]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %w", err)
	}

	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	return &Config{
		Host:    host,
		Port:    port,
		Timeout: timeout,
	}, nil
}

// connect устанавливает TCP-соединение с указанным хостом и портом,
// используя заданный таймаут.
func connect(cfg *Config) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// startIO запускает двунаправленный обмен данными между STDIN/STDOUT и соединением.
// Эта функция не возвращает управление до завершения сеанса.
func startIO(conn net.Conn) {
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() {
		once.Do(func() {
			close(done)
		})
	}

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				// Пишем ровно столько байт, сколько прочитали
				if _, writeErr := os.Stdout.Write(buf[:n]); writeErr != nil {
					closeDone()
					return
				}
			}
			if err != nil {
				closeDone()
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if _, writeErr := conn.Write(buf[:n]); writeErr != nil {
					closeDone()
					return
				}
			}
			if err == io.EOF {
				closeDone()
				return
			}
			if err != nil {
				closeDone()
				return
			}
		}
	}()

	<-done
	conn.Close()
}

func main() {
	cfg, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	conn, err := connect(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	startIO(conn)
}
