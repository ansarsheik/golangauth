package main

import (
	"context"
	"database/sql"
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

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql"
)

const (
	StatusSuccess = "success"
	StatusError   = "error"
	StatusFail    = "fail"
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

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func respondwithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	//fmt.Println(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

type Product struct {
	Id 			int `json:"pid"`
	Title 		string `json:"Product_Title"`
	Category 	string `json:"Product_Category"`
	Type 		string `json:"Product_Type"`
	Websale 	string `json:"Websale"`
	Psid 		int `json:"psid"`
	Wcrypt 		string `json:"Weightcrypt"`
	Weight 		string `json:"Weight"`
	Metal 		string `json:"Metal"`
	Rpid		int		`json:"rpid"`
} //TODO use Capital letter for first word in structs

type Quantity struct {
	QtyFrom int `json:"qtyFrom"`
	QtyTo	int `json:"qtyTo"`
	Price	string `json:Price`
	Psid	int `json:"psid"`
}

type JsonSend struct {
	Products Product `json:"product"`
	Quantity []Quantity `json:"quantity"`
	SimilarProducts	[]Product `json:"similarproducts"`
}

func getProducts(w http.ResponseWriter, r *http.Request) {

	db := DBconnection()

	category := r.URL.Query().Get("category")
	productType := r.URL.Query().Get("type")
	random := r.URL.Query().Get("random")
	limit := r.URL.Query().Get("limit")
	productId := r.URL.Query().Get("productId")

	var cond string

	if len(category) > 0 {
		cond = " AND p.Product_Category= '" + category + "'"
	}

	if len(productType) > 0 {
		cond += " AND p.Product_Type= '" + productType + "'"
	}

	if len(productId) > 0 {
		cond += " AND ps.psid != "+ productId
	}

	query := "SELECT " +
		"p.pid," +
		"p.Product_Title," +
		"p.Product_Category, " +
		"p.Product_Type, " +
		"SUM(((ps.MarginPC/100)*ps.SellPrice)+ps.SellPrice) as `Websale`, " +
		"ps.psid," +
		"ENCRYPT(ps.Weight) AS Wcrypt, "+
		"ps.Weight AS Weight, " +
		"ps.Metal " +
		"FROM products p " +
		"INNER JOIN products_stock ps ON ps.primary_rpid = p.rpid " +
		"WHERE 1 " + cond +
		" GROUP by p.pid "

	if len(random) > 0 {
		query += " order by RAND() "
	}

	if len(limit) > 0 {
		query += " limit "+ limit
	}

	fmt.Println(query)

	results, err := db.Query(query)
	if err != nil {
		panic(err.Error())
	}

	defer results.Close()

	var products = []Product{}

	for results.Next() {

		var p Product
		err = results.Scan(
			&p.Id,
			&p.Title,
			&p.Category,
			&p.Type,
			&p.Websale,
			&p.Psid,
			&p.Wcrypt,
			&p.Weight,
			&p.Metal)

		if err != nil {
			log.Println(err.Error())
		}
		products = append(products, p)
		//fmt.Println(p)
		//w.Write([]byte(fmt.Sprintf("Userid : ", userid)))
	}

	//output, _ := json.MarshalIndent(products,"","  ")

	respondwithJSON(w,201,products)
}

var myClient = &http.Client{Timeout: 10 * time.Second}

func getProductPage(w http.ResponseWriter, r *http.Request) {

	db := DBconnection()

	productId := r.URL.Query().Get("productId")

	var cond string

	if len(productId) > 0 {
		cond = " AND ps.StockInSale = 'Y' AND ps.psid=" + productId
	}

	//fmt.Println(cond)
	//fmt.Println(productId)

	query := "SELECT " +
		"p.pid," +
		"p.Product_Title," +
		"p.Product_Category, " +
		"p.Product_Type, " +
		"SUM(((ps.MarginPC/100)*ps.SellPrice)+ps.SellPrice) as `Websale`, " +
		"ps.psid," +
		"ENCRYPT(ps.Weight) AS Wcrypt, "+
		"ps.Weight AS Weight, " +
		"ps.Metal, " +
		"p.rpid " +
		" FROM products p " +
		" INNER JOIN products_stock ps ON ps.child_rpid = p.rpid " +
		" WHERE 1 " + cond + " limit 1"

	//fmt.Println(query)

	results, err := db.Query(query)
	if err != nil {
		panic(err.Error())
	}

	defer results.Close()

	prod := Product{}
	for results.Next() {
		var id, psid, rpid int
		var title, category, ptype, websale,wcrypt,weight, metal string

		err = results.Scan(
			&id,
			&title,
			&category,
			&ptype,
			&websale,
			&psid,
			&wcrypt,
			&weight,
			&metal,
			&rpid)

		if err != nil {
			log.Println(err.Error())
		}

		prod.Id = id
		prod.Title = title
		prod.Category = category
		prod.Type = ptype
		prod.Websale = websale
		prod.Psid = psid
		prod.Wcrypt = wcrypt
		prod.Weight = weight
		prod.Metal = metal
		prod.Rpid = rpid
	}

	qtyquery := "SELECT ps.qtyFrom, ps.qtyTo, ps.Price, ps.psid " +
		"from products_stock ps " +
		"WHERE ps.primary_rpid = ?"

	qtyresults, err := db.Query(qtyquery, prod.Rpid)
	if err != nil {
		panic(err.Error())
	}

	defer qtyresults.Close()

	var qty = []Quantity{}

	for qtyresults.Next() {

		var q Quantity
		err = qtyresults.Scan(
			&q.QtyFrom,
			&q.QtyTo,
			&q.Price,
			&q.Psid)

		if err != nil {
			log.Println(err.Error())
		}

		qty = append(qty, q)
	}

	//output, _ := json.MarshalIndent(products,"","  ")
	//fmt.Println(qty)
	//fmt.Println(prod)

	url := "http://192.168.1.5:9999/products?limit=6&productId="+productId+"&category="+prod.Category+"&type="+prod.Type // just pass the file name

	//fmt.Println(url)
	res, err := http.Get(url)
	if err != nil {
		panic(err.Error())
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err.Error())
	}

	var sm []Product
	json.Unmarshal(body, &sm)

	//fmt.Println(sm)

	var jsend = JsonSend {
		prod,
		qty,
		sm}

	respondwithJSON(w,201, jsend)
}

func DBconnection() (db *sql.DB) {
	dbDriver := "mysql"
	dbUser := os.Getenv("MYSQL_USER")
	dbPassword := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_DATABASE")

	// when using containers - no matter what port mysql is mapped to it uses only service name to depend on...
	//db, err := sql.Open(dbDriver, dbUser+":"+dbPassword+"@tcp(mysqlserver:3306)/"+dbName)
	// to test in dev environment
	db, err := sql.Open(dbDriver, dbUser+":"+dbPassword+"@tcp(127.0.0.1:3350)/"+dbName+"?parseTime=true")
	if err != nil {
		log.Println(err.Error())
	}

	return db
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()

	e := godotenv.Load() //Load .env file
	if e != nil {
		log.Println(e.Error())
	}

	r := mux.NewRouter()

	//r.Use(commonMiddleware)
	// end points
	r.HandleFunc("/products", getProducts).Methods("GET")
	r.HandleFunc("/productpage", getProductPage).Methods("GET")

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + os.Getenv("WEBSERVER_PORT"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Println("Starting Webserver at port " + os.Getenv("WEBSERVER_PORT"))
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
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