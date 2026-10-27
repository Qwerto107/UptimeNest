package notify

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Qwerto107/UptimeNest/internal/model"
)

func TestSMTPMessageEncodingAndRecipientDelivery(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	message := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		fmt.Fprint(conn, "220 localhost SMTP\r\n")
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverErrors <- err
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(conn, "250 localhost\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				fmt.Fprint(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "DATA"):
				fmt.Fprint(conn, "354 continue\r\n")
				var body strings.Builder
				for {
					line, err = reader.ReadString('\n')
					if err != nil {
						serverErrors <- err
						return
					}
					if line == ".\r\n" {
						break
					}
					body.WriteString(line)
				}
				message <- body.String()
				fmt.Fprint(conn, "250 accepted\r\n")
			case strings.HasPrefix(line, "QUIT"):
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				serverErrors <- fmt.Errorf("unexpected command: %s", line)
				return
			}
		}
	}()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	v := model.NotificationConfig{Host: "127.0.0.1", Port: port, Security: "none", From: "UptimeNest <monitor@example.com>", To: []string{"ops@example.com", "admin@example.com"}}
	if err = sendMail(context.Background(), v, "服务告警\r\nInjected: no", "中文正文\nsecond line"); err != nil {
		t.Fatal(err)
	}
	select {
	case raw := <-message:
		parsed, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Header.Get("Injected") != "" || !strings.Contains(parsed.Header.Get("To"), "admin@example.com") {
			t.Fatalf("bad headers: %+v", parsed.Header)
		}
		data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, parsed.Body))
		if err != nil || string(data) != "中文正文\nsecond line" {
			t.Fatalf("encoding: %q %v", data, err)
		}
	case err := <-serverErrors:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("SMTP did not receive mail")
	}
}
