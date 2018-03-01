package main

import (
	"bufio"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	login    = "login"
	password = "pass"
	logger   *log.Logger
)

var noMessage = fmt.Errorf("No incoming message")

type incoming struct {
	Msg []struct {
		SmsId        int    `xml:"sms_id,attr"`
		Sender       string `xml:"sender,attr"`
		Destination  int    `xml:"destination,attr"`
		DateReceived string `xml:"date_received,attr"`
		Text         string `xml:",innerxml"`
	} `xml:"msg,omitempty"`
}

type packageXML struct {
	XMLName xml.Name `xml:"package"`
	//Login    string     `xml:"login,attr"`
	//Password string     `xml:"password,attr"`
	incoming []incoming `xml:"incoming,omitempty"`
}

func readSMS() (packageXML, error) {
	var (
		err  error
		req  *http.Request
		resp *http.Response
		body []byte
	)

	var pkg packageXML

	xmlText := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8" ?><package login="%s" password="%s"><incoming><get_msg/><msg/></incoming></package>`, login, password)

	xmlReader := strings.NewReader(xmlText)

	conn := &http.Client{
		Timeout: time.Second * 10,
	}

	if req, err = http.NewRequest("POST", "http://service.craftmobile.ru/", xmlReader); err != nil {
		return pkg, err
	}

	req.Header.Add("Content-Type", "application/xml; charset=utf-8")
	if resp, err = conn.Do(req); err != nil {
		return pkg, err
	}
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return pkg, err
	}

	defer resp.Body.Close()

	err = xml.Unmarshal(body, &pkg)

	return pkg, err
}

func newCsvWriter(w io.Writer, bom bool) *csv.Writer {
	bw := bufio.NewWriter(w)
	if bom {
		bw.Write([]byte{0xEF, 0xBB, 0xBF})
	}
	return csv.NewWriter(bw)
}

func writeCSV(incoming incoming) error {
	var (
		err     error
		fileCSV *os.File
	)

	nameCSV := time.Now().Format("2006.01.02.csv")
	if fileCSV, err = os.OpenFile(nameCSV, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644); err != nil {
		return err
	}
	defer fileCSV.Close()

	r := csv.NewReader(fileCSV)
	r.Comma = ';'
	r.LazyQuotes = true

	read, err := r.ReadAll()
	if err != nil {
		return err
	}

	sliceWripper := [][]string{}
	if len(read) == 0 {
		sliceWripper = append(sliceWripper, []string{"Sms_id", "Sender", "Destination", "Date_received", "Message"})
	}

	if len(incoming.Msg) < 1 {
		return noMessage
	}

	writerFile := newCsvWriter(fileCSV, true)
	defer writerFile.Flush()
	writerFile.Comma = ';'

	for _, val := range incoming.Msg {
		slice := []string{strconv.Itoa(val.SmsId), val.Sender, strconv.Itoa(val.Destination), val.DateReceived, val.Text}
		sliceWripper = append(sliceWripper, slice)
	}

	for _, value := range sliceWripper {
		if err = writerFile.Write(value); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	l, err := os.OpenFile("log_file.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("error opening utm log file: %v", err)
	}
	defer l.Close()
	multi := io.MultiWriter(l, os.Stdout)
	logger = log.New(multi, "", log.Ldate|log.Ltime)

	logger.Print("Start program")
	for {
		pkg, err := readSMS()
		if err != nil {
			logger.Println(err)
		}
		if len(pkg.incoming) == 0 {
			fmt.Println("Новых сообщений нет")
			break
		}

		if len(pkg.incoming[0].Msg) != 0 {
			fmt.Printf("Количество полученных сообщений: %d\n", len(pkg.incoming[0].Msg))
			err = writeCSV(pkg.incoming[0])
			if err != nil {
				logger.Fatal(err)
			}
		}
	}

}
