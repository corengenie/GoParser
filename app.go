package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type UrlResponse struct {
	//Ответ от сервера с полем url куда отправлялся запрос
	url      string
	response string
}

func main() {
	flag.String("urlsFile", "", "файл с urls")
	flag.String("resultsFolder", "", "папка с результатами")
	flag.Parse()

	now := time.Now()

	logFile, err := os.Create(now.Format("02.01.2006 15-04") + ".log")
	check(err)
	defer logFile.Close()

	urls, err := os.Open(flag.Arg(0))
	check(err)
	defer urls.Close()

	os.Mkdir(flag.Arg(1), os.FileMode(0644))
	os.Mkdir(flag.Arg(1)+"_2", os.FileMode(0644))

	scanner := bufio.NewScanner(urls)
	readedURLS := make([]string, 0)

	var wg sync.WaitGroup
	defer wg.Wait()

	fmt.Print("Чтение файла\n")

	for scanner.Scan() {
		url := scanner.Text()
		readedURLS = append(readedURLS, url)
	}

	fmt.Println("Данные прочтены")
	fmt.Println("Начало выполнения без горутин")

	doWithout := func() {
		for _, url := range readedURLS {
			logmsg("Парсинг "+url, logFile)
			fmt.Printf("Парсинг %s\n", url)
			resp, err := getHTML(url)
			resultURL := strings.Split(url, "/")[2]
			logmsg("Создание файла для "+url, logFile)
			f, err := os.Create(flag.Arg(1) + "_2" + "/" + resultURL + ".html")
			check(err)
			logmsg("Запись файла в "+flag.Arg(1)+"/"+resultURL, logFile)
			f.Write(resp)
			fmt.Print("Запись в файл " + resultURL + "\n")
		}
	}

	start := time.Now()
	doWithout()
	endWithout := time.Since(start)
	fmt.Println("Начало выполнения с горутинами")

	start = time.Now()
	urlsChan := make(chan string)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer println("Данные отправлены\n")
		defer close(urlsChan)
		for _, url := range readedURLS {
			logmsg("Отправка "+url+" на парсинг", logFile)
			fmt.Printf("Отправка на парсинг %s\n", url)
			urlsChan <- url
		}
	}()

	parsedURLS := make(chan UrlResponse)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(parsedURLS)
		defer println("Данные распарсены\n")
		for {
			gotURL, ok := <-urlsChan
			if !ok {
				break
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				fmt.Printf("Начало парсинга %s\n", gotURL)
				logmsg("Выполнение запроса к "+gotURL, logFile)
				result, err := getHTML(gotURL)
				if err != nil {
					fmt.Println("Ошибка при запросе к "+gotURL+"\nОшибка: %e", err)
					logmsg("Ошибка при запросе к "+gotURL+"\n", logFile)
				}
				fmt.Printf("Конец парсинга %s\n", gotURL)
				resultURL := strings.Split(gotURL, "/")[2]
				parsedURLS <- UrlResponse{resultURL, string(result)}
			}()
		}
		<-time.After(2 * time.Second)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer println("Данные сохранены\n")
		for {
			value, ok := <-parsedURLS
			if !ok {
				break
			}
			wg.Add(1)
			go func(data UrlResponse) {
				defer wg.Done()
				logmsg("Создание файла для "+data.url, logFile)
				f, err := os.Create(flag.Arg(1) + "/" + data.url + ".html")
				check(err)
				defer f.Close()
				logmsg("Запись файла в "+flag.Arg(1)+"/"+data.url, logFile)
				f.Write([]byte(data.response))
				fmt.Print("Запись в файл для " + data.url + "\n")
			}(value)
		}
	}()

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
	wg.Wait()

	endWith := time.Since(start)

	fmt.Printf("Время без горутин: %s\n", endWithout.String())
	fmt.Printf("Время с горутинами: %s\n", endWith.String())
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func logmsg(msg string, logFile *os.File) {
	logFile.WriteString(time.Now().Format("02.01.2006 15-04") + " " + msg + "\n")
}

func getHTML(url string) ([]byte, error) {
	client := http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		n := make([]byte, 0)
		return n, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}
