package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type Сurrency struct {
	Name  string  `json:"Name"`
	Value float64 `json:"Value"`
}

type DailyRate struct {
	Date   string              `json:"Date"`
	Valute map[string]Сurrency `json:"Valute"`
}

func fetch(date time.Time, out chan<- DailyRate, errCh chan<- error) {
	url := fmt.Sprintf("https://www.cbr-xml-daily.ru/archive/%v/daily_json.js", date.Format("2006/01/02"))
	resp, err := http.Get(url)
	if err != nil {
		errCh <- fmt.Errorf("Error GET query for %v: %w", date.Format("2006/01/02"), err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		errCh <- fmt.Errorf("StatuseCode %d is not 200 for %v", resp.StatusCode, date.Format("2006/01/02"))
		return
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		errCh <- fmt.Errorf("Error while reading response Body for %v: %w", date.Format("2006/01/02"), err)
		return
	}
	var res DailyRate
	err = json.Unmarshal(data, &res)
	if err != nil {
		errCh <- fmt.Errorf("Error while unmarshling response Body for %v: %w", date.Format("2006/01/02"), err)
		return
	}
	out <- res
}

func main() {
	days := 90   // Количество дней для запроса
	workers := 5 // Количество одновременных запросов

	dates := make(chan time.Time, days)
	results := make(chan DailyRate, days)
	errCh := make(chan error, days)

	var wg sync.WaitGroup

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, days*(-1))

	// Генерирование дат
	go func() {
		for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
			if date.Weekday() != time.Saturday && date.Weekday() != time.Sunday {
				dates <- date
			}
		}
		close(dates)
	}()

	// Запуск запросов
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for date := range dates {
				fetch(date, results, errCh)
			}
		}()
	}
	// Ожидание получения всех результатов
	go func() {
		wg.Wait()
		close(results)
		close(errCh)
	}()

	type ExtremeValue struct {
		Name  string
		Date  string
		Value float64
	}
	maxVal := make(map[string]ExtremeValue)
	minVal := make(map[string]ExtremeValue)
	sumСurrency := 0.0
	countСurrency := 0
	errorsCount := 0
	daysCount := 0

	// Обработка результата
Loop:
	for {
		select {
		case res, ok := <-results:
			if !ok {
				break Loop
			}
			daysCount++
			for name, currency := range res.Valute {
				if _, exist := maxVal[name]; !exist {
					maxVal[name] = ExtremeValue{
						Name:  currency.Name,
						Date:  res.Date,
						Value: currency.Value,
					}
					minVal[name] = ExtremeValue{
						Name:  currency.Name,
						Date:  res.Date,
						Value: currency.Value,
					}
				}

				if maxVal[name].Value < currency.Value {
					maxVal[name] = ExtremeValue{
						Name:  currency.Name,
						Date:  res.Date,
						Value: currency.Value,
					}
				}
				if minVal[name].Value > currency.Value {
					minVal[name] = ExtremeValue{
						Name:  currency.Name,
						Date:  res.Date,
						Value: currency.Value,
					}
				}
				sumСurrency += currency.Value
				countСurrency++
			}
		case err, ok := <-errCh:
			if !ok {
				break Loop
			}
			log.Println(err)
			errorsCount++
		}
	}

	// Вывод результатов

	// 1. Максимальные значения
	fmt.Println("Максимальные курсы валют:")
	for code, extreme := range maxVal {
		fmt.Printf("%s (%s): %.4f руб. (дата: %s)\n",
			code, extreme.Name, extreme.Value, extreme.Date)
	}

	// 2. Минимальные значения
	fmt.Println("\nМинимальные курсы валют:")
	for code, extreme := range minVal {
		fmt.Printf("%s (%s): %.4f руб. (дата: %s)\n",
			code, extreme.Name, extreme.Value, extreme.Date)
	}

	// 3. Среднее значение курса рубля
	fmt.Printf("\nСредний курс рубля за период: %.4f\n", sumСurrency/float64(countСurrency))

	fmt.Printf("\nОбработано дней: %d\nОшибок: %d\n", daysCount, errorsCount)
}
