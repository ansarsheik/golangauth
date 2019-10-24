package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gamegos/jsend"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting down")
	os.Exit(0)
}

func respondwithJSON(w http.ResponseWriter, code int, payload interface{}) {

	jsend.Wrap(w).
		Status(code).
		Data(payload).
		Send()

}

var myClient = &http.Client{Timeout: 10 * time.Second}

func checkErr(err error) bool {
	log.Println(err)
	if err != nil {
		return true
	}
	return false
}

type BasketProduct struct {
	Psid int `json:"psid"`
	Qty  int `json:"qty"`
}

type Basket struct {
	BasketData []BasketProduct `json:"basketdata"`
}

func print(msg interface{}) {
	//fmt.Println(msg)
}

func getResponseBody(r *http.Request) Basket {
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	checkErr(err)

	//fmt.Println(r.Body)

	var basket Basket
	err = json.Unmarshal(b, &basket)
	checkErr(err)

	//fmt.Println(basket)
	return basket
}

func getPrimaryId(psid int) int {
	db := DBconnection()
	query := "SELECT primary_rpid FROM products_stock where psid = ?"

	psidresults, err := db.Query(query, psid)
	checkErr(err)

	defer psidresults.Close()

	prod := Product{}
	for psidresults.Next() {
		var psid int

		err = psidresults.Scan(
			&psid,
		)

		checkErr(err)

		prod.Psid = psid
	}

	return prod.Psid
}

func getPrimaryPsid(psid int) int {
	db := DBconnection()
	query := "select ps.psid from products_stock ps " +
		"inner join products_stock ps1 on ps1.primary_rpid = ps.primary_rpid " +
		"where ps1.psid = ? order by ps.qtyfrom asc limit 1"

	psidresults, err := db.Query(query, psid)
	checkErr(err)

	defer psidresults.Close()

	prod := Product{}
	for psidresults.Next() {
		var psid int

		err = psidresults.Scan(
			&psid,
		)

		checkErr(err)

		prod.Psid = psid
	}

	return prod.Psid
}

func getSellableProduct(psid int, qty int) Product {

	query := "SELECT " +
		"p.pid," +
		"p.Product_Title," +
		"p.Product_Category, " +
		"p.Product_Type, " +
		"SUM(((ps.MarginPC/100)*ps.SellPrice)+ps.SellPrice) as `Websale`, " +
		"SUM(((ps.MarginPC/100)*ps.SellPrice)+ps.SellPrice) as `Subtotal`, " +
		"p.pid as Quantity , " +
		"p.pid as PrimaryPsid , " +
		"ps.psid," +
		"ENCRYPT(ps.Weight) AS Wcrypt, " +
		"ps.Weight AS Weight, " +
		"ps.Metal, " +
		"p.rpid " +
		"FROM products_stock ps " +
		" INNER JOIN products p ON p.rpid = ps.primary_rpid " +
		" WHERE ps.primary_rpid = ? " +
		"AND ? BETWEEN ps.qtyfrom AND ps.qtyto"

	db := DBconnection()

	//fmt.Println(query)

	//fmt.Println("line 128", getPrimaryId(psid))

	prodResults, err := db.Query(query, getPrimaryId(psid), qty)
	checkErr(err)
	defer prodResults.Close()

	prod := Product{}
	for prodResults.Next() {
		var id, psid, rpid, primarypsid, quantity int
		var websale, subtotal float32
		var title, category, ptype, wcrypt, weight, metal string

		err = prodResults.Scan(
			&id,
			&title,
			&category,
			&ptype,
			&websale,
			&subtotal,
			&quantity,
			&primarypsid,
			&psid,
			&wcrypt,
			&weight,
			&metal,
			&rpid)

		checkErr(err)

		prod.Id = id
		prod.Title = title
		prod.Category = category
		prod.Type = ptype
		prod.Websale = websale
		prod.Subtotal = subtotal * float32(qty)
		prod.Quantity = qty
		prod.PrimaryPsid = getPrimaryPsid(psid)
		prod.PrimaryPsid = primarypsid
		prod.Psid = psid
		prod.Wcrypt = wcrypt
		prod.Weight = weight
		prod.Metal = metal
		prod.Rpid = rpid
	}

	//fmt.Println("line 165 ", prod)

	return prod
}

func getBasketData(w http.ResponseWriter, r *http.Request) {
	//var basketData Basket
	var basketProducts = getResponseBody(r)
	//fmt.Println(basketProducts)
	var basketProductResult []Product
	for _, product := range basketProducts.BasketData {
		//fmt.Println(product.Psid)
		basketProductResult = append(basketProductResult, getSellableProduct(product.Psid, product.Qty))
	}

	//fmt.Println("line 185 ", basketProductResult)
	respondwithJSON(w, 200, basketProductResult)
}

func test(w http.ResponseWriter, r *http.Request) {
	fmt.Println(os.Getenv("MYSQL_PASSWORD"))
}

func main() {
	flag.Parse()
	e := godotenv.Load() //Load .env file
	checkErr(e)
	r := mux.NewRouter()
	//r.Use(commonMiddleware)
	// end points
	r.HandleFunc("/test", test).Methods("GET")
	r.HandleFunc("/products", getProducts).Methods("GET")
	r.HandleFunc("/productpage/{productId}", getProductPage).Methods("GET")
	r.HandleFunc("/minmax/{productId}", getProductMinMax).Methods("GET")
	//r.HandleFunc("/menuitems", getMenuItems).Methods("GET")

	var port string
	if len(os.Getenv("WEBSERVER_PORT")) < 1 {
		port = "9990"
	} else {
		port = os.Getenv("WEBSERVER_PORT")
	}

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	// Start Server
	go func() {
		log.Println("Starting Webserver at port " + port)
		if err := srv.ListenAndServe(); err != nil {
			checkErr(err)
		}
	}()
	// Graceful Shutdown
	waitForShutdown(srv)
}

/*
router.GET("/product/:pid", Productpage)
func Productpage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fmt.Fprintf(w, "productid %s", ps.ByName("pid"))
}
*/
