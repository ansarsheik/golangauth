package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type Product struct {
	Id          int     `json:"pid"`
	Title       string  `json:"Product_Title"`
	Category    string  `json:"Product_Category"`
	Type        string  `json:"Product_Type"`
	Websale     float32 `json:"Websale"`
	Subtotal    float32 `json:"subtotal"`
	Quantity    int     `json:"qty"`
	PrimaryPsid int     `json:"p_psid"`
	Psid        int     `json:"psid"`
	Wcrypt      string  `json:"Weightcrypt"`
	Weight      string  `json:"Weight"`
	Metal       string  `json:"Metal"`
	Rpid        int     `json:"rpid"`
} //TODO use Capital letter for first word in structs

type ProductFilters struct {
	Category, Type, Random, Search, SortBy string
	Limit, ProductId                       string
}

type WeightDataFilter struct {
	WeightInGrams []WeightFilter `json:"weight_in_grams"`
	WeightInOz    []WeightFilter `json:"weight_in_oz"`
}

type WeightFilter struct {
	Weight      float32 `json:"weight"`
	Ustring     string  `json:"ustring"`
	WeightHuman string  `json:"weight_human"`
}

type ProductFeedData struct {
	Products []Product        `json:"products"`
	Filters  WeightDataFilter `json:"filters"`
}

type ProductPage struct {
	Products        Product    `json:"product"`
	Quantity        []Quantity `json:"quantity"`
	SimilarProducts []Product  `json:"similarproducts"`
}

type MinMax struct {
	Minqty int `json:"min"`
	Maxqty int `json:"max"`
}

type ErrorMessage struct {
	Message string `json:"message"`
}

type Quantity struct {
	QtyFrom int    `json:"qtyFrom"`
	QtyTo   int    `json:"qtyTo"`
	Price   string `json:"price"`
	Psid    int    `json:"psid"`
}

func filterProductsForMenu(Type string, Category string) []Product {

	db := DBconnection()

	query := "SELECT " +
		"p.Product_Title," +
		"ps.psid as Psid" +
		" FROM products p " +
		" INNER JOIN products_stock ps ON ps.primary_rpid = p.rpid " +
		" WHERE p.Product_Type = '" + Type + "'" +
		" AND p.Product_Category = '" + Category + "' " +
		" AND ps.StockInSale = 'Y' " +
		" ORDER BY RAND() LIMIT 4 "

	results, err := db.Query(query)
	defer results.Close()

	var menu []Product

	for results.Next() {
		var m Product
		err = results.Scan(
			&m.Title,
			&m.Psid)

		checkErr(err)
		menu = append(menu, m)
	}

	return menu
}

func prepareCondition(filter ProductFilters) string {
	var cond string
	if len(filter.Category) > 0 {
		cond = " AND p.Product_Category= '" + filter.Category + "'"
	}

	if len(filter.Type) > 0 {
		cond += " AND p.Product_Type= '" + filter.Type + "'"
	}

	if len(filter.ProductId) > 0 {
		cond += " AND ps.psid != " + filter.ProductId
	}

	//fmt.Println("one",filter.Search)

	if len(filter.Search) > 0 {
		fmt.Println(filter.Search)

		search := strings.Split(filter.Search, "+")

		//fmt.Println(len(search))

		if len(search) > 1 {
			cond += " AND (p.Product_Type like '%" + search[0] + "%' AND p.Product_Category like '%" + search[1] + "%' )"
			cond += " or (p.Product_Category like '%" + search[0] + "%' AND p.Product_Type like '%" + search[1] + "%' )"

			searchtext := strings.Replace(filter.Search, "+", " ", 2)
			//fmt.Println("two",searchtext)
			cond += " or p.Product_Title like '%" + searchtext + "%'"
		} else {
			cond += " AND p.Product_Title like '%" + filter.Search + "%' or (p.Product_Type like '%" + filter.Search +
				"%' or p.Product_Category like '%" + filter.Search + "%' )"
		}
	}

	//fmt.Println(cond)

	return cond
}

func getWeightFilter(wt string, filter ProductFilters) []WeightFilter {

	db := DBconnection()
	//weight filters
	weightGms := "SELECT DISTINCT " +
		"round( ps.Weight,2 ) as `weight`,ENCRYPT(ps.Weight) as `ustring`, ps.WeightHuman as `weighthuman` " +
		"FROM products_stock ps " +
		"LEFT JOIN products p ON p.rpid = ps.child_rpid " +
		"WHERE 1 " + prepareCondition(filter) +
		" AND ps.WeightHuman = '" + wt +
		"' HAVING weight > 0 " +
		"ORDER BY `weight` ASC"

	fmt.Println(weightGms)

	resultsGms, err := db.Query(weightGms)
	checkErr(err)

	defer resultsGms.Close()

	var weightinGms []WeightFilter

	for resultsGms.Next() {
		var w WeightFilter
		err = resultsGms.Scan(
			&w.Weight,
			&w.Ustring,
			&w.WeightHuman)

		checkErr(err)
		weightinGms = append(weightinGms, w)
	}

	return weightinGms
}

func getProductsFeed(filter ProductFilters) []Product {

	query := "SELECT " +
		"p.pid," +
		"p.Product_Title," +
		"p.Product_Category, " +
		"p.Product_Type, " +
		"ps.SellPrice AS Websale, " +
		"ps.psid," +
		"ENCRYPT(ps.Weight) AS Wcrypt, " +
		"ps.Weight AS Weight, " +
		"ps.Metal " +
		"FROM products p " +
		"INNER JOIN products_stock ps ON ps.child_rpid = p.rpid " +
		"WHERE 1 AND ps.StockInSale = 'Y' " + prepareCondition(filter) +
		" GROUP BY p.pid, ps.SellPrice, ps.psid"

	if len(filter.Random) > 0 {
		query += " order by RAND() "
	}

	if len(filter.Limit) > 0 {
		query += " limit " + filter.Limit
	}

	if len(filter.SortBy) > 0 {
		if filter.SortBy == "lowest" {
			query += " order by ps.SellPrice ASC"
		} else {
			query += " order by ps.SellPrice DESC"
		}
	}

	//fmt.Println(query)
	db := DBconnection()
	results, err := db.Query(query)
	checkErr(err)

	defer results.Close()

	var products []Product

	if !results.Next() {
		return products
	}

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

		checkErr(err)
		products = append(products, p)
	}

	return products
}

func isInt(productId string) bool {
	match, _ := regexp.MatchString("([0-9]+)", productId)
	return match
}

func getProductMinMax(w http.ResponseWriter, r *http.Request) {
	db := DBconnection()

	vars := mux.Vars(r)
	productId := vars["productId"]

	match := isInt(productId)
	if !match {
		error := ErrorMessage{
			"Product Id required"}
		respondwithJSON(w, 400, error)
		return
	}

	priquery := "select primary_rpid from products_stock where psid = ? limit 1"
	priresults, err := db.Query(priquery, productId)
	if checkErr(err) {
		error := ErrorMessage{
			"Error Finding Product"}
		respondwithJSON(w, 400, error)
		return
	}

	defer priresults.Close()
	var rpid_val int

	for priresults.Next() {
		var rpid int
		err = priresults.Scan(&rpid)
		checkErr(err)
		rpid_val = rpid
	}

	if rpid_val == 0 {
		error := ErrorMessage{
			"Error Finding Product"}
		respondwithJSON(w, 400, error)
		return
	}

	minmaxQuery := "select max(ps.qtyto) as maxi,min(ps.qtyfrom) as mini " +
		"from products_stock ps " +
		"inner join products p on p.rpid = ps.primary_rpid " +
		"where ps.primary_rpid = ?"

	minmaxResults, err := db.Query(minmaxQuery, rpid_val)
	checkErr(err)

	defer minmaxResults.Close()
	var qtymaxn, qtyminn int
	for minmaxResults.Next() {
		var qtymax, qtymin int
		err = minmaxResults.Scan(&qtymax, &qtymin)
		checkErr(err)
		qtymaxn = qtymax
		qtyminn = qtymin
	}

	var minmax = MinMax{
		qtyminn,
		qtymaxn}

	respondwithJSON(w, 200, minmax)
}

func getProducts(w http.ResponseWriter, r *http.Request) {

	fmt.Println(r.URL.Query().Get("search"))

	filters := ProductFilters{
		Category:  r.URL.Query().Get("category"),
		Type:      r.URL.Query().Get("type"),
		Random:    r.URL.Query().Get("random"),
		Limit:     r.URL.Query().Get("limit"),
		Search:    r.URL.Query().Get("search"),
		SortBy:    r.URL.Query().Get("sort"),
		ProductId: r.URL.Query().Get("productId")}

	products := getProductsFeed(filters)
	if len(products) < 1 {
		error := ErrorMessage{
			"Error Getting Product Feed"}
		respondwithJSON(w, 400, error)
		return
	}

	weightInGrams := getWeightFilter("gms", filters)
	weightInOz := getWeightFilter("oz", filters)

	filterdata := WeightDataFilter{
		WeightInGrams: weightInGrams,
		WeightInOz:    weightInOz,
	}

	data := ProductFeedData{
		Products: products,
		Filters:  filterdata,
	}
	//output, _ := json.MarshalIndent(products,"","  ")
	respondwithJSON(w, 200, data)
}

func getProductPage(w http.ResponseWriter, r *http.Request) {

	db := DBconnection()
	vars := mux.Vars(r)
	productId := vars["productId"]
	limit := r.URL.Query().Get("limit")
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
		"ENCRYPT(ps.Weight) AS Wcrypt, " +
		"ps.Weight AS Weight, " +
		"ps.Metal, " +
		"p.rpid " +
		" FROM products p " +
		" INNER JOIN products_stock ps ON ps.child_rpid = p.rpid " +
		" WHERE 1 " + cond + " limit 1"

	//fmt.Println(query)
	results, err := db.Query(query)
	checkErr(err)

	defer results.Close()

	if !results.Next() {
		error := ErrorMessage{
			"Error Loading Product Page"}
		respondwithJSON(w, 400, error)
		return
	}

	prod := Product{}
	for results.Next() {
		var id, psid, rpid int
		var websale float32
		var title, category, ptype, wcrypt, weight, metal string

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

		checkErr(err)

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
	checkErr(err)

	defer qtyresults.Close()

	var qty []Quantity

	for qtyresults.Next() {

		var q Quantity
		err = qtyresults.Scan(
			&q.QtyFrom,
			&q.QtyTo,
			&q.Price,
			&q.Psid)

		checkErr(err)

		qty = append(qty, q)
	}

	//output, _ := json.MarshalIndent(products,"","  ")
	//fmt.Println(qty)
	//fmt.Println(prod)
	if len(limit) < 1 {
		limit = strconv.FormatInt(int64(6), 10)
	}

	//strconv.FormatInt(int64(prod.Rpid), 10)
	similarProductFilters := ProductFilters{
		ProductId: productId,
		Limit:     limit,
		Category:  prod.Category,
		Type:      prod.Type}

	similarProducts := getProductsFeed(similarProductFilters)

	var productpage = ProductPage{
		prod,
		qty,
		similarProducts}

	respondwithJSON(w, 201, productpage)
}
