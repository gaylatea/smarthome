package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

var (
	addr = flag.String("addr", "localhost:19406", "drops server to connect to")

	// ssl options
	caCert  = flag.String("caCert", "ca.crt", "Only clients signed with this CA will be accepted")
	sslCert = flag.String("sslCert", "server.crt", "SSL certificate to present to clients")
	sslKey  = flag.String("sslKey", "server.key", "SSL private key to load")

	uids = map[string]chan []string{}
)

func pump(secs int) error {
	glog.Infof("pumping water for %ds.", secs)
	return nil
}

func generateUID() string {
	return "asdf123"
}

func init() {
	flag.Set("logtostderr", "true")
}

func main() {
	flag.Parse()

	// setup the ssl socket
	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(*sslCert, *sslKey)
	if err != nil {
		glog.Fatalf("could not load server key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(*caCert)
	if err != nil {
		glog.Fatalf("could not read ca certificate: %s", err)
	}

	// Append the client certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		glog.Fatalf("failed to append client certs")
	}

	// Create the TLS credentials
	creds := &tls.Config{
		ClientAuth:               tls.RequireAndVerifyClientCert,
		Certificates:             []tls.Certificate{certificate},
		RootCAs:                  certPool,
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", *addr, creds)
	if err != nil {
		glog.Fatalf("couldn't connect to the drops server: %v", err)
	}
	defer conn.Close()

	glog.Infof("Starting server.")
	connReader := bufio.NewReader(conn)

	uid := generateUID()
	send := fmt.Sprintf("%s\n", fmt.Sprintf("%s REGISTER water source", uid))
	if _, err := conn.Write([]byte(send)); err != nil {
		glog.Fatalf("couldn't send registration: %v", err)
	}
	output, err := connReader.ReadString('\n')
	if err != nil {
		glog.Fatalf("couldn't send registration: %v", err)
	}
	expect := fmt.Sprintf("%s ACK\n", uid)
	if output != expect {
		glog.Fatalf("registration failed: %v", err)
	}

	glog.Infof("Registration complete.")

	go func() {
		// report water measurements
		for {
			uid := generateUID()
			c := make(chan []string, 0)
			uids[uid] = c
			defer func() {
				delete(uids, uid)
			}()

			level := rand.Float64()
			send := fmt.Sprintf("%s\n", fmt.Sprintf("%s METRIC level %f", uid, level))
			if _, err := conn.Write([]byte(send)); err != nil {
				glog.Fatalf("couldn't send metric: %v", err)
			}

			ret := <-c
			if ret[0] != "ACK\n" {
				glog.Errorf("return failed failed: %s", output)
			}

			time.Sleep(10 * time.Second)
		}
	}()

	for {
		output, err := connReader.ReadString('\n')
		if err != nil {
			glog.Fatalf("couldn't read from conn: %v", err)
		}

		cmdParts := strings.Split(output, " ")
		glog.Infof("got %v", cmdParts)
		if len(cmdParts) < 2 {
			glog.Errorf("bad line %s", output)
			continue
		}
		uid := cmdParts[0]
		if ch, ok := uids[uid]; ok {
			glog.Infof("client exists for uid %s", uid)
			ch <- cmdParts[1:]
			continue
		}

		cmd := cmdParts[1]
		go func(uid, cmd string, args ...string) {
			c := make(chan []string, 0)
			uids[uid] = c
			defer func() {
				delete(uids, uid)
			}()

			if len(args) != 2 {
				conn.Write([]byte(fmt.Sprintf("%s ERR\n", uid)))

				ret := <-c
				if ret[0] != "ACK\n" {
					glog.Errorf("return failed failed: %s", output)
				}
				return
			}

			if cmd != "RUN" {
				glog.Infof("unknown cmd %s", cmd)
				conn.Write([]byte(fmt.Sprintf("%s ERR\n", uid)))

				ret := <-c
				if ret[0] != "ACK\n" {
					glog.Errorf("return failed failed: %s", output)
				}
				return
			}

			fnName, arg := args[0], args[1]
			if fnName != "pump" {
				glog.Infof("unknown fn %s", fnName)
				conn.Write([]byte(fmt.Sprintf("%s ERR\n", uid)))

				ret := <-c
				if ret[0] != "ACK\n" {
					glog.Errorf("return failed failed: %s", output)
				}
				return
			}

			arg = strings.TrimSuffix(arg, "\n")
			intArg, err := strconv.Atoi(arg)
			if err != nil {
				glog.Infof("couldn't convert %s to int: %v", arg, err)
				conn.Write([]byte(fmt.Sprintf("%s ERR\n", uid)))

				ret := <-c
				if ret[0] != "ACK\n" {
					glog.Errorf("return failed failed: %s", output)
				}
				return
			}

			if err := pump(intArg); err != nil {
				glog.Infof("error pumping: %v", err)
				conn.Write([]byte(fmt.Sprintf("%s ERR\n", uid)))

				ret := <-c
				if ret[0] != "ACK\n" {
					glog.Errorf("return failed failed: %s", output)
				}
				return
			}

			conn.Write([]byte(fmt.Sprintf("%s DONE\n", uid)))

			ret := <-c
			if ret[0] != "ACK\n" {
				glog.Errorf("return success failed: %s", output)
			}
		}(uid, cmd, cmdParts[2:]...)
	}
}
