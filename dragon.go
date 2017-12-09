package main

import (
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis"

	but "github.com/achillesss/but4print"
	"github.com/parnurzeal/gorequest"
)

type Config struct {
	CoinIDs []int `toml:"coinIDs"`
}

type cryptoCurrency struct {
	CoinID int     `json:"coin_id"`
	Name   string  `json:"name"`
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

type dragonex struct {
	Exchange       float64           `json:"exchange"`
	Coins          []*cryptoCurrency `json:"coins"`
	DTDay          int               `json:"dt_day"`
	DTPeriod       int               `json:"dt_period"`
	DTTodayRelease float64           `json:"dt_today_release"`
	DTTotalRelease float64           `json:"dt_total_release"`
}

type params map[string]string

var d dragonex
var debugOn = flag.Bool("debug", false, "debug")
var exchange float64
var DT_START_DATE = time.Date(2017, time.October, 31, 16, 0, 0, 1, time.UTC)
var redisClient *redis.Client
var timeZone = time.FixedZone("BeiJing", 8*3600)

// 今日为龙币发行的第几天
func dtDay(day time.Time) int {
	d := day.Sub(DT_START_DATE).Hours() / 24
	if float64(int(d)) > d {
		d++
	}
	return int(d)
}

// 今日为龙币发行的第几个阶段
func dtPeriod(day int) int {
	return (day-1)/365 + 1
}

// 计算某阶段龙币发行了多少天
func dtPeriodDay(day, period int) int {
	periodDay := day % 365
	if day > period*365 {
		periodDay = 365
	}
	return periodDay
}

// 计算某天应该发行的龙币数量
func dtTodayRelease(day, period int) float64 {
	return 51200 * math.Pow(0.5, float64(period-1))
}

// 龙币在某阶段应该发型了多少
func dtPeriodRelease(day, period int) float64 {
	periodDay := dtPeriodDay(day, period)
	todayRelease := dtTodayRelease(day, period)
	return float64(periodDay) * todayRelease
}

// 今日应该发型的龙币数量
func dtTotalRelease(day, period int) float64 {
	var total float64
	for i := 0; i < period; i++ {
		total += dtPeriodRelease(day, i+1)
	}
	return total
}

func debug(format string, args ...interface{}) {
	if *debugOn {
		but.NewButer(nil, format, args...).Color(but.COLOR_CYAN, false).Print()
	}
}

func logInfo(format string, args ...interface{}) {
	but.NewButer(nil, format, args...).Color(but.COLOR_BLUE, false).Print()
}

func logWarn(format string, args ...interface{}) {
	but.NewButer(nil, format, args...).Color(but.COLOR_YELLOW, false).Print()
}

func logErr(format string, args ...interface{}) {
	but.NewButer(nil, format, args...).Color(but.COLOR_RED, false).Print()
}

func makeRequest(method, url string, p params) *gorequest.SuperAgent {
	req := gorequest.New().CustomMethod(method, url)
	for k, v := range p {
		req.Param(k, v)
	}
	return req
}

func listCoin() {
	type coinlistData struct {
		Code   string  `json:"code"`
		Name   string  `json:"name"`
		CoinID int     `json:"coin_id"`
		Price  float64 `json:"price,string"`
	}
	type coinList struct {
		OK   bool            `json:"ok"`
		Msg  string          `json:"msg"`
		Code int             `json:"code"`
		Data []*coinlistData `json:"data"`
	}
	var resp coinList
	makeRequest("GET", "https://a.dragonex.io/coin/list/", nil).EndStruct(&resp)
	if resp.OK {
		var cids []int
		for _, data := range resp.Data {
			cids = append(cids, data.CoinID)
			var c cryptoCurrency
			c.CoinID = data.CoinID
			c.Name = strings.ToUpper(data.Code)
			c.Price = data.Price
			d.updateCoinData(c)
		}
		updateCoinDetail(cids...)
	}
}

func updateCoinDetail(coinIDs ...int) bool {
	type marketData struct {
		CoinID      int     `json:"coin_id,string"`
		TotalAmount float64 `json:"total_amount,string"`
		ClosePrice  float64 `json:"close_price,string"`
	}
	type market struct {
		Code int           `json:"code"`
		OK   bool          `json:"ok"`
		Msg  string        `json:"msg,omitempty"`
		Data []*marketData `json:"data"`
	}
	var coinIDSlice []string
	for _, cid := range coinIDs {
		coinIDSlice = append(coinIDSlice, fmt.Sprintf("%d", cid))
	}

	p := params{
		"coin_ids": strings.Join(coinIDSlice, ","),
		"time":     fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	var m market
	_, _, err := makeRequest("GET", "https://a.dragonex.io/market/real/", p).Set("Content-Type", "application/json").EndStruct(&m)

	if err != nil {
		logErr("get market failed. error: %v\n", err)
		return false
	}

	if !m.OK {
		logWarn("get market not ok. msg: %s\n", m.Msg)
		return false
	}

	for _, data := range m.Data {
		if data.CoinID == data.CoinID {
			var c cryptoCurrency
			c.CoinID = data.CoinID
			c.Volume = data.TotalAmount * 2
			d.updateCoinData(c)
		}
	}
	return true
}

func (c *cryptoCurrency) update(src cryptoCurrency) {
	if src.CoinID != 0 {
		c.CoinID = src.CoinID
	}
	if src.Name != "" {
		c.CoinID = src.CoinID
	}
	if src.Price > 0 {
		c.Price = src.Price
	}
	if src.Volume > 0 {
		c.Volume = src.Volume
	}
}

func (d *dragonex) getCoin(coinID int) *cryptoCurrency {
	for _, cc := range d.Coins {
		if cc.CoinID == coinID {
			return cc
		}
	}
	return nil
}

func (d *dragonex) updateCoinData(c cryptoCurrency) {
	var updated bool

	if cc := d.getCoin(c.CoinID); cc != nil {
		cc.update(c)
		updated = true
	}

	if !updated {
		d.Coins = append(d.Coins, &c)
	}
}

func (d *dragonex) updateDTdetail() {
	d.DTDay = dtDay(time.Now())
	d.DTPeriod = dtPeriod(d.DTDay)
	d.DTTodayRelease = dtTodayRelease(d.DTDay, d.DTPeriod)
	d.DTTotalRelease = dtTotalRelease(d.DTDay, d.DTPeriod)
}

func task(hour, min, sec int) {
	for {
		t := time.Now().UTC()
		start := time.Date(t.Year(), t.Month(), t.Day(), hour, min, sec, 0, time.UTC)
		if t.After(start) {
			start = start.AddDate(0, 0, 1)
		}
		ticker := time.NewTimer(start.Sub(t))
		<-ticker.C
		logInfo("开始统计今日数据\n")
		d.updateDTdetail()
		listCoin()
		logInfo("统计完成\n")
		d.writeData(start.In(timeZone).Format("2006-01-02"))
	}
}

func totalAmountKey(date string) string {
	return key("totalamount", date)
}

func totalDTReleaseKey(date string) string {
	return key("totalrelease", date)
}

func dtHighCostReleaseKey(date string) string {
	return key("dthigh", date)
}

func dtLowCostReleaseKey(date string) string {
	return key("dtlow", date)
}

func dtBonus(date string) string {
	return key("dtbonus", date)
}

func key(prefix, date string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s", prefix, date))
}

func (d *dragonex) totalAmountCNY() float64 {
	var total float64
	for _, data := range d.Coins {
		total += data.Volume
	}
	return total * exchange
}

func (d *dragonex) writeData(date string) {
	totalAmount := d.totalAmountCNY()
	logInfo("开始写入今日数据（%s）\n", date)
	redisClient.Set(totalAmountKey(date), fmt.Sprintf("%.4f", totalAmount), 0)
	redisClient.Set(totalDTReleaseKey(date), fmt.Sprintf("%.4f", d.DTTotalRelease), 0)
	redisClient.Set(dtHighCostReleaseKey(date), fmt.Sprintf("%4f", dtCost(d.DTTodayRelease, totalAmount, 0.3)), 0)
	redisClient.Set(dtLowCostReleaseKey(date), fmt.Sprintf("%4f", dtCost(d.DTTodayRelease, totalAmount, 0.5)), 0)
	redisClient.Set(dtBonus(date), fmt.Sprintf("%.4f", income(totalAmount)/d.DTTotalRelease), 0)
	logInfo("写入完成\n")
}

func setupRedis() {
	opts := new(redis.Options)
	opts.Addr = "localhost:6379"
	opts.DB = 1
	redisClient = redis.NewClient(opts)
	res, err := redisClient.Ping().Result()
	if err != nil {
		panic(fmt.Sprintf("setup redis failed. error: %v\n", err))
	}
	logInfo("redis response: %v\n", res)
}

func init() {
	flag.Parse()
	logInfo("初始化数据中...\n")
	setupRedis()
	exchange = 6.5
	d.updateDTdetail()
	listCoin()
	logInfo("初始化完毕\n")
	go func() {
		t := time.NewTicker(time.Second * 2)
		for {
			<-t.C
			d.updateDTdetail()
			listCoin()
		}
	}()

	var sumHour = 15
	var sumMin = 59
	var sumSec = 55
	go task(sumHour, sumMin, sumSec)
}

func dtCost(dtAmount float64, exchangeAmount float64, rate float64) float64 {
	return income(exchangeAmount) / dtAmount / rate
}

func income(amount float64) float64 {
	return 0.002 * amount
}
func (d *dragonex) String() string {
	headformat := "%10s%20s%20s%20s%20s\n"
	bodyFormat := "%10s%20.4f%20.4f%20.4f%20.4f\n"
	str := fmt.Sprintf(headformat, "Coin", "Price($)", "Price(¥)", "24h Amount($)", "24h Amount(¥)")
	var totalAmount float64
	for _, coin := range d.Coins {
		totalAmount += coin.Volume
		str += fmt.Sprintf(bodyFormat, coin.Name, coin.Price, coin.Price*exchange, coin.Volume, coin.Volume*exchange)
	}
	str += fmt.Sprintf(bodyFormat, "Total", 0.0, 0.0, totalAmount, totalAmount*exchange)
	yestoday := time.Now().In(timeZone).AddDate(0, 0, -1)
	yestodayStr := yestoday.Format("2006-01-02")
	yesAmount, _ := strconv.ParseFloat(redisClient.Get(totalAmountKey(yestodayStr)).Val(), 64)
	if yesAmount != 0 {
		str += fmt.Sprintf(bodyFormat, "Yestoday", 0.0, 0.0, yesAmount/exchange, yesAmount)
	}
	str += fmt.Sprintf("----------\n")

	hc := dtCost(d.DTTodayRelease, totalAmount*exchange, 0.3)
	lc := dtCost(d.DTTodayRelease, totalAmount*exchange, 0.5)
	in := income(totalAmount * exchange)
	str += fmt.Sprintf("%10s%10s%15s%15s%15s%15s%15s\n", "DT:    Day", "Period", "Today Release", "Total Release", "Cost_H(¥)", "Cost_L(¥)", "Bonus(¥)")
	str += fmt.Sprintf("%10d%10d%15.4f%15.4f%15.4f%15.4f%15.4f\n", d.DTDay, d.DTPeriod, d.DTTodayRelease, d.DTTotalRelease, hc, lc, in/d.DTTotalRelease)

	dtYestoday := dtDay(yestoday)
	dtYestodayPeriod := dtPeriod(dtYestoday)
	dtYestodayRelease := dtTodayRelease(dtYestoday, dtYestodayPeriod)
	dtYestodayTotalRelease := dtTotalRelease(dtYestoday, dtYestodayPeriod)
	dtYestodayHighCost := dtCost(dtYestodayRelease, yesAmount, .3)
	dtYestodayLowCost := dtCost(dtYestodayRelease, yesAmount, .5)
	yestodayIncome := income(yesAmount)
	if yestodayIncome != 0 {
		str += fmt.Sprintf("%10s%10d%15.4f%15.4f%15.4f%15.4f%15.4f\n", "yestoday", dtYestodayPeriod, dtYestodayRelease, dtYestodayTotalRelease, dtYestodayHighCost, dtYestodayLowCost, yestodayIncome/dtYestodayTotalRelease)
	}

	str += fmt.Sprintf("==========\n")
	return str
}

func main() {
	tLog := time.NewTicker(time.Second * 5)
	for _ = range tLog.C {
		logInfo("dragonex.io Information (%s):\n%v\n", time.Now().Format("2006-01-02 15:04:05"), d.String())
	}
}
