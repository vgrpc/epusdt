package service

import (
	"fmt"
	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/mq"
	"github.com/assimon/luuu/mq/handle"
	"github.com/assimon/luuu/telegram"
	"github.com/assimon/luuu/util/http_client"
	"github.com/assimon/luuu/util/json"
	"github.com/assimon/luuu/util/log"
	"github.com/assimon/luuu/util/oklink"
	"github.com/golang-module/carbon/v2"
	"github.com/gookit/goutil/stdutil"
	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type UsdtTrc20Resp struct {
	PageSize int    `json:"page_size"`
	Code     int    `json:"code"`
	Data     []Data `json:"data"`
}

type OkTransactionResp struct {
	Code string   `json:"code"`
	Data []OkData `json:"data"`
}

type OkLinkV1Resp struct {
	Code      int      `json:"code"`
	Data      OkV1Data `json:"data"`
	Msg       string   `json:"msg"`
	DetailMsg string   `json:"detailMsg"`
}
type OkV1Data struct {
	Hits []OkV1Transaction `json:"hits"`
}
type OkData struct {
	TransactionList []OkTransaction `json:"transactionList"`
}

type OkTransaction struct {
	Txid                 string `json:"txId"`
	BlockHash            string `json:"blockHash"`
	Height               string `json:"height"`
	TransactionTime      string `json:"transactionTime"`
	From                 string `json:"from"`
	To                   string `json:"to"`
	TokenContractAddress string `json:"tokenContractAddress"`
	TokenId              string `json:"tokenId"`
	Amount               string `json:"amount"`
	Symbol               string `json:"symbol"`
}

type OkV1Transaction struct {
	Txid                 string  `json:"txhash"`
	BlockHash            string  `json:"blockHash"`
	Height               int64   `json:"blockHeight"`
	BlockTime            int64   `json:"blocktime"`
	From                 string  `json:"from"`
	To                   string  `json:"to"`
	TokenContractAddress string  `json:"tokenContractAddress"`
	Amount               float64 `json:"value"`
	Symbol               string  `json:"symbol"`
}

type TokenInfo struct {
	TokenID      string `json:"tokenId"`
	TokenAbbr    string `json:"tokenAbbr"`
	TokenName    string `json:"tokenName"`
	TokenDecimal int    `json:"tokenDecimal"`
	TokenCanShow int    `json:"tokenCanShow"`
	TokenType    string `json:"tokenType"`
	TokenLogo    string `json:"tokenLogo"`
	TokenLevel   string `json:"tokenLevel"`
	IssuerAddr   string `json:"issuerAddr"`
	Vip          bool   `json:"vip"`
}

type Data struct {
	Amount         string `json:"amount"`
	ApprovalAmount string `json:"approval_amount"`
	BlockTimestamp int64  `json:"block_timestamp"`
	Block          int    `json:"block"`
	From           string `json:"from"`
	To             string `json:"to"`
	Hash           string `json:"hash"`
	Confirmed      int    `json:"confirmed"`
	ContractType   string `json:"contract_type"`
	ContracTType   int    `json:"contractType"`
	Revert         int    `json:"revert"`
	ContractRet    string `json:"contract_ret"`
	EventType      string `json:"event_type"`
	IssueAddress   string `json:"issue_address"`
	Decimals       int    `json:"decimals"`
	TokenName      string `json:"token_name"`
	ID             string `json:"id"`
	Direction      int    `json:"direction"`
}

// Trc20CallBack trc20回调
func Trc20CallBack(token string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Error(err)
		}
	}()
	client := http_client.GetHttpClient()
	startTime := carbon.Now().AddHours(-24).TimestampWithMillisecond()
	endTime := carbon.Now().TimestampWithMillisecond()
	resp, err := client.R().SetQueryParams(map[string]string{
		"sort":            "-timestamp",
		"limit":           "50",
		"start":           "0",
		"direction":       "2",
		"db_version":      "1",
		"trc20Id":         config.UsdtContractAddress,
		"address":         token,
		"start_timestamp": stdutil.ToString(startTime),
		"end_timestamp":   stdutil.ToString(endTime),
	}).Get(config.GetTronApiUri())
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != http.StatusOK {
		panic(err)
	}
	var trc20Resp UsdtTrc20Resp
	err = json.Cjson.Unmarshal(resp.Body(), &trc20Resp)
	if err != nil {
		panic(err)
	}
	if trc20Resp.PageSize <= 0 {
		return
	}
	for _, transfer := range trc20Resp.Data {
		if transfer.To != token || transfer.ContractRet != "SUCCESS" {
			continue
		}
		decimalQuant, err := decimal.NewFromString(transfer.Amount)
		if err != nil {
			panic(err)
		}
		decimalDivisor := decimal.NewFromFloat(1000000)
		amount := decimalQuant.Div(decimalDivisor).InexactFloat64()
		tradeId, err := data.GetTradeIdByWalletAddressAndAmount(token, amount)
		if err != nil {
			panic(err)
		}
		if tradeId == "" {
			continue
		}
		order, err := data.GetOrderInfoByTradeId(tradeId)
		if err != nil {
			panic(err)
		}
		// 区块的确认时间必须在订单创建时间之后
		createTime := order.CreatedAt.TimestampWithMillisecond()
		if transfer.BlockTimestamp < createTime {
			panic("Orders cannot actually be matched")
		}
		// 到这一步就完全算是支付成功了
		req := &request.OrderProcessingRequest{
			Token:              token,
			TradeId:            tradeId,
			Amount:             amount,
			BlockTransactionId: transfer.Hash,
		}
		err = OrderProcessing(req)
		if err != nil {
			panic(err)
		}
		// 回调队列
		orderCallbackQueue, _ := handle.NewOrderCallbackQueue(order)
		mq.MClient.Enqueue(orderCallbackQueue, asynq.MaxRetry(5))
		// 发送机器人消息
		msgTpl := `
<b>📢📢有新的交易支付成功！</b>
<pre>交易号：%s</pre>
<pre>订单号：%s</pre>
<pre>请求支付金额：%f cny</pre>
<pre>实际支付金额：%f usdt</pre>
<pre>钱包地址：%s</pre>
<pre>订单创建时间：%s</pre>
<pre>支付成功时间：%s</pre>
`
		msg := fmt.Sprintf(msgTpl, order.TradeId, order.OrderId, order.Amount, order.ActualAmount, order.Token, order.CreatedAt.ToDateTimeString(), carbon.Now().ToDateTimeString())
		telegram.SendToBot(msg)
	}
}

// Trc20CallBack trc20回调
func Trc20CallBackByOklink(token string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Error(err)
		}
	}()
	client := http_client.GetHttpClient()

	resp, err := client.R().SetQueryParams(map[string]string{
		"chainShortName":       "TRON",
		"address":              token,
		"protocolType":         "token_20",
		"tokenContractAddress": config.UsdtContractAddress,
		"limit":                "50",
	}).SetHeader("OK-ACCESS-KEY", config.OklinkKey).Get("https://www.oklink.com/api/v5/explorer/address/token-transaction-list")
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != http.StatusOK {
		panic(err)
	}
	var okLinkResp OkTransactionResp
	err = json.Cjson.Unmarshal(resp.Body(), &okLinkResp)
	if err != nil {
		panic(err)
	}
	if len(okLinkResp.Data) <= 0 {
		return
	}
	for _, transfer := range okLinkResp.Data[0].TransactionList {
		if transfer.To != token {
			continue
		}
		decimalQuant, err := decimal.NewFromString(transfer.Amount)
		if err != nil {
			panic(err)
		}
		amount := decimalQuant.InexactFloat64()
		tradeId, err := data.GetTradeIdByWalletAddressAndAmount(token, amount)
		if err != nil {
			panic(err)
		}
		if tradeId == "" {
			continue
		}
		order, err := data.GetOrderInfoByTradeId(tradeId)
		if err != nil {
			panic(err)
		}
		// 区块的确认时间必须在订单创建时间之后
		createTime := order.CreatedAt.TimestampWithMillisecond()
		transactionTime, err := strconv.ParseInt(transfer.TransactionTime, 10, 64)
		if err != nil {
			return
		}
		if transactionTime < createTime {
			panic("Orders cannot actually be matched")
		}
		// 到这一步就完全算是支付成功了
		req := &request.OrderProcessingRequest{
			Token:              token,
			TradeId:            tradeId,
			Amount:             amount,
			BlockTransactionId: transfer.Txid,
		}
		err = OrderProcessing(req)
		if err != nil {
			panic(err)
		}
		// 回调队列
		orderCallbackQueue, _ := handle.NewOrderCallbackQueue(order)
		mq.MClient.Enqueue(orderCallbackQueue, asynq.MaxRetry(5))
		// 发送机器人消息
		msgTpl := `
<b>📢📢有新的交易支付成功！</b>
<pre>交易号：%s</pre>
<pre>订单号：%s</pre>
<pre>请求支付金额：%f cny</pre>
<pre>实际支付金额：%f usdt</pre>
<pre>钱包地址：%s</pre>
<pre>订单创建时间：%s</pre>
<pre>支付成功时间：%s</pre>
`
		msg := fmt.Sprintf(msgTpl, order.TradeId, order.OrderId, order.Amount, order.ActualAmount, order.Token, order.CreatedAt.ToDateTimeString(), carbon.Now().ToDateTimeString())
		telegram.SendToBot(msg)
	}
}

// Trc20CallBack trc20回调
func Trc20CallBackByOklinkExplorerApiV1(token string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Error(err)
		}
	}()
	client := http_client.GetHttpClient()

	resp, err := client.R().SetQueryParams(map[string]string{
		"offset":          "0",
		"isError":         "success",
		"address":         token,
		"tokenType":       "TRC20",
		"contractAddress": config.UsdtContractAddress,
		"limit":           "20",
		"nonzeroValue":    "true",
		"t":               strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
	}).SetHeader("X-Apikey", oklink.GetApiKey()).SetHeader("X-Utc", "8").Get("https://www.oklink.com/api/explorer/v1/tron/transfers")
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != http.StatusOK {
		panic(err)
	}
	var okLinkResp OkLinkV1Resp
	err = json.Cjson.Unmarshal(resp.Body(), &okLinkResp)
	if err != nil {
		panic(err)
	}
	for _, transfer := range okLinkResp.Data.Hits {
		if transfer.To != token {
			continue
		}

		amount := transfer.Amount
		tradeId, err := data.GetTradeIdByWalletAddressAndAmount(token, amount)
		if err != nil {
			panic(err)
		}
		if tradeId == "" {
			continue
		}
		order, err := data.GetOrderInfoByTradeId(tradeId)
		if err != nil {
			panic(err)
		}
		// 区块的确认时间必须在订单创建时间之后
		createTime := order.CreatedAt.TimestampWithMillisecond()
		transactionTime := transfer.BlockTime * 1000
		if transactionTime < createTime {
			panic("Orders cannot actually be matched")
		}
		// 到这一步就完全算是支付成功了
		req := &request.OrderProcessingRequest{
			Token:              token,
			TradeId:            tradeId,
			Amount:             amount,
			BlockTransactionId: transfer.Txid,
		}
		err = OrderProcessing(req)
		if err != nil {
			panic(err)
		}
		// 回调队列
		orderCallbackQueue, _ := handle.NewOrderCallbackQueue(order)
		mq.MClient.Enqueue(orderCallbackQueue, asynq.MaxRetry(5))
		// 发送机器人消息
		msgTpl := `
<b>📢📢有新的交易支付成功！</b>
<pre>交易号：%s</pre>
<pre>订单号：%s</pre>
<pre>请求支付金额：%f cny</pre>
<pre>实际支付金额：%f usdt</pre>
<pre>钱包地址：%s</pre>
<pre>订单创建时间：%s</pre>
<pre>支付成功时间：%s</pre>
`
		msg := fmt.Sprintf(msgTpl, order.TradeId, order.OrderId, order.Amount, order.ActualAmount, order.Token, order.CreatedAt.ToDateTimeString(), carbon.Now().ToDateTimeString())
		telegram.SendToBot(msg)
	}
}
