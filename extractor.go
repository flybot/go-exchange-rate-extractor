package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	_ "github.com/lib/pq"
)

type XeRates struct {
	currency string
	rate     string
}

func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

// Scrape all exchange rates for selected date
func ScrapeXE(date string) []XeRates {
	// Request the HTML page.
	res, err := http.Get("https://www.xe.com/currencytables/?from=DKK&date=" + date + "#table-section")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var result []XeRates

	// Find the items
	doc.Find("tbody").First().Find("tr").Each(func(i int, s *goquery.Selection) {
		currency := s.Find("a").Text()
		if currency == "" {
			currency = s.Find("th").First().Text()
		}
		rate := s.Find("td").Last().Text()
		var xe XeRates
		xe.currency = currency
		xe.rate = rate
		result = append(result, xe)
	})
	return result
}

func main() {

	// open database
	db, err := sql.Open("postgres", "host=localhost port=5432 user=root password=secret dbname=devdb sslmode=disable")
	CheckError(err)

	// close database
	defer db.Close()

	// check db
	err = db.Ping()
	CheckError(err)

	// load empty-rate rows
	selectQuery := `select entry_date, currency from (select entry_date, currency, rate from public.receipts 
		left join public.currency_exchange_rates ON public.receipts.entry_date = public.currency_exchange_rates.date AND public.receipts.currency = public.currency_exchange_rates.from
	where currency <> 'DKK' group by entry_date, currency, rate) a where a.rate is null and a.entry_date IS not null order by a.entry_date`
	rows, err := db.Query(selectQuery)
	CheckError(err)
	defer rows.Close()

	for rows.Next() {
		var entry_date string
		var currency string

		err = rows.Scan(&entry_date, &currency)
		CheckError(err)

		//get list of currencies
		r := ScrapeXE(entry_date)
		for _, elem := range r {
			if elem.currency == currency {
				//fmt.Printf("%s : %s\n", elem.currency, elem.rate)
				insertDynStmt := `insert into public.currency_exchange_rates("from", "to", rate, date) values($1, 'DKK', $2, $3)`
				_, e := db.Exec(insertDynStmt, elem.currency, elem.rate, entry_date)
				CheckError(e)

				break
			}
		}
	}

}
