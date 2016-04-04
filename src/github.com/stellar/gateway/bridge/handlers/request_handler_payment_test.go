package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/facebookgo/inject"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stellar/gateway/bridge/config"
	"github.com/stellar/gateway/horizon"
	"github.com/stellar/gateway/mocks"
	"github.com/stellar/gateway/protocols/federation"
	"github.com/stellar/gateway/protocols/stellartoml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequestHandlerPayment(t *testing.T) {
	mockHorizon := new(mocks.MockHorizon)
	mockTransactionSubmitter := new(mocks.MockTransactionSubmitter)
	mockFederationResolver := new(mocks.MockFederationResolver)
	mockStellartomlResolver := new(mocks.MockStellartomlResolver)

	requestHandler := RequestHandler{
		Config: &config.Config{
			NetworkPassphrase: "Test SDF Network ; September 2015",
		},
	}

	// Inject mocks
	var g inject.Graph

	err := g.Provide(
		&inject.Object{Value: &requestHandler},
		&inject.Object{Value: mockHorizon},
		&inject.Object{Value: mockTransactionSubmitter},
		&inject.Object{Value: mockFederationResolver},
		&inject.Object{Value: mockStellartomlResolver},
	)
	if err != nil {
		panic(err)
	}

	if err := g.Populate(); err != nil {
		panic(err)
	}

	testServer := httptest.NewServer(http.HandlerFunc(requestHandler.Payment))
	defer testServer.Close()

	Convey("Given payment request", t, func() {
		Convey("When source is invalid", func() {
			source := "SDRAS7XIQNX25UDCCX725R4EYGBFYGJE4HJ2A3DFCWJIHMRSMS7CXX43"

			Convey("it should return error", func() {
				statusCode, response := getResponse(testServer, url.Values{"source": {source}})
				responseString := strings.TrimSpace(string(response))
				assert.Equal(t, 400, statusCode)
				expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentInvalidSource}
				assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
			})
		})

		Convey("When destination is invalid", func() {
			source := "SDRAS7XIQNX25UDCCX725R4EYGBFYGJE4HJ2A3DFCWJIHMRSMS7CXX42"
			destination := "GD3YBOYIUVLU"

			mockFederationResolver.On(
				"Resolve",
				"GD3YBOYIUVLU",
			).Return(
				federation.Response{AccountId: "GD3YBOYIUVLU"},
				stellartoml.StellarToml{},
				nil,
			).Once()

			Convey("it should return error", func() {
				statusCode, response := getResponse(testServer, url.Values{"destination": {destination}, "source": {source}})
				responseString := strings.TrimSpace(string(response))
				assert.Equal(t, 400, statusCode)
				expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentInvalidDestination}
				assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
			})
		})

		Convey("When destination is a Stellar address", func() {
			params := url.Values{
				"source":      {"SDRAS7XIQNX25UDCCX725R4EYGBFYGJE4HJ2A3DFCWJIHMRSMS7CXX42"},
				"destination": {"bob*stellar.org"},
			}

			Convey("When FederationResolver returns error", func() {
				mockFederationResolver.On(
					"Resolve",
					"bob*stellar.org",
				).Return(
					federation.Response{},
					stellartoml.StellarToml{},
					errors.New("stellar.toml response status code indicates error"),
				).Once()

				Convey("it should return error", func() {
					statusCode, response := getResponse(testServer, params)
					responseString := strings.TrimSpace(string(response))
					assert.Equal(t, 400, statusCode)
					expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentCannotResolveDestination}
					assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
				})
			})

			Convey("When federation response is correct (no memo)", func() {
				validParams := url.Values{
					// GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ
					"source":      {"SDWLS4G3XCNIYPKXJWWGGJT6UDY63WV6PEFTWP7JZMQB4RE7EUJQN5XM"},
					"destination": {"bob*stellar.org"},
					"amount":      {"20"},
				}

				mockFederationResolver.On(
					"Resolve",
					"bob*stellar.org",
				).Return(
					federation.Response{AccountId: "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
					stellartoml.StellarToml{},
					nil,
				).Once()

				// Checking if destination account exists
				mockHorizon.On(
					"LoadAccount",
					"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
				).Return(horizon.AccountResponse{}, nil).Once()

				// Loading sequence number
				mockHorizon.On(
					"LoadAccount",
					"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
				).Return(
					horizon.AccountResponse{
						SequenceNumber: "100",
					},
					nil,
				).Once()

				var ledger uint64
				ledger = 1988728
				horizonResponse := horizon.SubmitTransactionResponse{&ledger, nil, nil}

				mockHorizon.On(
					"SubmitTransaction",
					"AAAAAIu7VxM5f9eQ3va0bpvKprxnSHB4zyEnY4D/VzT8Jio3AAAAZAAAAAAAAABlAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAA5IVbm6A8mbgc/apAizxmBf4zZmqbedR3Ke+MTa7pjVYAAAAAAAAAAAvrwgAAAAAAAAAAAfwmKjcAAABAh3M6y9LXiWD0GB1KCkgNS5H1Lnyr1wS1BsfzoM1/v0muzobwNkJinV+RcWyC8VfeKqOjKBOANJnEusl+sHkcAg==",
				).Return(horizonResponse, nil).Once()

				Convey("it should return success", func() {
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 200, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})
			})

			Convey("When federation response is correct (with memo)", func() {
				validParams := url.Values{
					// GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ
					"source":      {"SDWLS4G3XCNIYPKXJWWGGJT6UDY63WV6PEFTWP7JZMQB4RE7EUJQN5XM"},
					"destination": {"bob*stellar.org"},
					"amount":      {"20"},
				}

				memoType := "text"
				memo := "125"

				mockFederationResolver.On(
					"Resolve",
					"bob*stellar.org",
				).Return(
					federation.Response{
						AccountId: "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
						MemoType:  &memoType,
						Memo:      &memo,
					},
					stellartoml.StellarToml{},
					nil,
				).Once()

				// Checking if destination account exists
				mockHorizon.On(
					"LoadAccount",
					"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
				).Return(horizon.AccountResponse{}, nil).Once()

				// Loading sequence number
				mockHorizon.On(
					"LoadAccount",
					"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
				).Return(
					horizon.AccountResponse{
						SequenceNumber: "100",
					},
					nil,
				).Once()

				var ledger uint64
				ledger = 1988728
				horizonResponse := horizon.SubmitTransactionResponse{&ledger, nil, nil}

				mockHorizon.On(
					"SubmitTransaction",
					"AAAAAIu7VxM5f9eQ3va0bpvKprxnSHB4zyEnY4D/VzT8Jio3AAAAZAAAAAAAAABlAAAAAAAAAAEAAAADMTI1AAAAAAEAAAAAAAAAAQAAAADkhVuboDyZuBz9qkCLPGYF/jNmapt51Hcp74xNrumNVgAAAAAAAAAAC+vCAAAAAAAAAAAB/CYqNwAAAEAjnc8Wf31VxgBXXhEmZfLo6c4YJtROVy5MTLsWFSx7TCkoQzCskBVcC30DrjQq7Vzm0zwg+mBmSGI5wFbctKgB",
				).Return(horizonResponse, nil).Once()

				Convey("it should return success", func() {
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 200, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})
			})
		})

		Convey("When assetIssuer is invalid", func() {
			source := "SDRAS7XIQNX25UDCCX725R4EYGBFYGJE4HJ2A3DFCWJIHMRSMS7CXX42"
			destination := "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"
			assetCode := "USD"
			assetIssuer := "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN631"

			mockFederationResolver.On(
				"Resolve",
				"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
			).Return(
				federation.Response{AccountId: "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
				stellartoml.StellarToml{},
				nil,
			).Once()

			Convey("it should return error", func() {
				statusCode, response := getResponse(
					testServer,
					url.Values{
						"source":       {source},
						"destination":  {destination},
						"asset_code":   {assetCode},
						"asset_issuer": {assetIssuer},
					},
				)
				responseString := strings.TrimSpace(string(response))
				assert.Equal(t, 400, statusCode)
				expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentInvalidIssuer}
				assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
			})
		})

		Convey("When assetCode is invalid", func() {
			// GBKGH7QZVCZ2ZA5OUGZSTHFNXTBHL3MPCKSCBJUAQODGPMWP7OMMRKDW
			source := "SDRAS7XIQNX25UDCCX725R4EYGBFYGJE4HJ2A3DFCWJIHMRSMS7CXX42"
			destination := "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"
			amount := "20"
			assetCode := "1234567890123"
			assetIssuer := "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"

			mockFederationResolver.On(
				"Resolve",
				"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
			).Return(
				federation.Response{AccountId: "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
				stellartoml.StellarToml{},
				nil,
			).Once()

			Convey("it should return error", func() {
				mockHorizon.On(
					"LoadAccount",
					"GBKGH7QZVCZ2ZA5OUGZSTHFNXTBHL3MPCKSCBJUAQODGPMWP7OMMRKDW",
				).Return(
					horizon.AccountResponse{
						SequenceNumber: "100",
					},
					nil,
				).Once()

				statusCode, response := getResponse(
					testServer,
					url.Values{
						"source":       {source},
						"destination":  {destination},
						"amount":       {amount},
						"asset_code":   {assetCode},
						"asset_issuer": {assetIssuer},
					},
				)
				responseString := strings.TrimSpace(string(response))
				assert.Equal(t, 400, statusCode)
				expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentMalformedAssetCode}
				assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
			})
		})

		Convey("When amount is invalid", func() {
			// GBKGH7QZVCZ2ZA5OUGZSTHFNXTBHL3MPCKSCBJUAQODGPMWP7OMMRKDW
			source := "SDRAS7XIQNX25UDCCX725R4EYGBFYGJE4HJ2A3DFCWJIHMRSMS7CXX42"
			destination := "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"
			amount := "test"
			assetCode := "USD"
			assetIssuer := "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"

			mockFederationResolver.On(
				"Resolve",
				"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
			).Return(
				federation.Response{AccountId: "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
				stellartoml.StellarToml{},
				nil,
			).Once()

			mockHorizon.On(
				"LoadAccount",
				"GBKGH7QZVCZ2ZA5OUGZSTHFNXTBHL3MPCKSCBJUAQODGPMWP7OMMRKDW",
			).Return(
				horizon.AccountResponse{
					SequenceNumber: "100",
				},
				nil,
			).Once()

			Convey("it should return error", func() {
				statusCode, response := getResponse(
					testServer,
					url.Values{
						"source":       {source},
						"destination":  {destination},
						"amount":       {amount},
						"asset_code":   {assetCode},
						"asset_issuer": {assetIssuer},
					},
				)
				responseString := strings.TrimSpace(string(response))
				assert.Equal(t, 400, statusCode)
				expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentInvalidAmount}
				assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
			})
		})

		Convey("When params are valid", func() {
			validParams := url.Values{
				// GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ
				"source":       {"SDWLS4G3XCNIYPKXJWWGGJT6UDY63WV6PEFTWP7JZMQB4RE7EUJQN5XM"},
				"destination":  {"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
				"amount":       {"20"},
				"asset_code":   {"USD"},
				"asset_issuer": {"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
			}

			mockFederationResolver.On(
				"Resolve",
				"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
			).Return(
				federation.Response{AccountId: "GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
				stellartoml.StellarToml{},
				nil,
			).Once()

			Convey("When memo is set", func() {
				Convey("only `memo` param is set", func() {
					validParams.Add("memo", "test")
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))
					assert.Equal(t, 400, statusCode)
					expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentMissingParamMemo}
					assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
				})

				Convey("only `memo_type` param is set", func() {
					validParams.Add("memo_type", "id")
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))
					assert.Equal(t, 400, statusCode)
					expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentMissingParamMemo}
					assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
				})

				Convey("memo_type=hash to long", func() {
					validParams.Add("memo_type", "hash")
					validParams.Add("memo", "012345678901234567890123456789012345678901234567890123456789012")
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))
					assert.Equal(t, 400, statusCode)
					expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentInvalidMemo}
					assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
				})

				Convey("unsupported memo_type", func() {
					validParams.Add("memo_type", "return_hash")
					validParams.Add("memo", "0123456789")
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))
					assert.Equal(t, 400, statusCode)
					expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentInvalidMemo}
					assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
				})

				Convey("memo is attached to the transaction", func() {
					mockHorizon.On(
						"LoadAccount",
						"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
					).Return(
						horizon.AccountResponse{
							SequenceNumber: "100",
						},
						nil,
					).Once()

					var ledger uint64
					ledger = 1988727
					horizonResponse := horizon.SubmitTransactionResponse{&ledger, nil, nil}

					mockHorizon.On(
						"SubmitTransaction",
						"AAAAAIu7VxM5f9eQ3va0bpvKprxnSHB4zyEnY4D/VzT8Jio3AAAAZAAAAAAAAABlAAAAAAAAAAIAAAAAAAAAewAAAAEAAAAAAAAAAQAAAADkhVuboDyZuBz9qkCLPGYF/jNmapt51Hcp74xNrumNVgAAAAFVU0QAAAAAAOSFW5ugPJm4HP2qQIs8ZgX+M2Zqm3nUdynvjE2u6Y1WAAAAAAvrwgAAAAAAAAAAAfwmKjcAAABADsRVwB27jfr3OthAWlRMSrxAIDPENw1dOfga5/cahnIneJQ5NPS5g96Rp8Y5xTsOU3Y9JmBDKB8g8lXFCXdwCA==",
					).Return(horizonResponse, nil).Once()

					validParams.Add("memo_type", "id")
					validParams.Add("memo", "123")
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 200, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})

				Convey("memo hash is attached to the transaction", func() {
					mockHorizon.On(
						"LoadAccount",
						"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
					).Return(
						horizon.AccountResponse{
							SequenceNumber: "100",
						},
						nil,
					).Once()

					var ledger uint64
					ledger = 1988727
					horizonResponse := horizon.SubmitTransactionResponse{&ledger, nil, nil}

					mockHorizon.On(
						"SubmitTransaction",
						"AAAAAIu7VxM5f9eQ3va0bpvKprxnSHB4zyEnY4D/VzT8Jio3AAAAZAAAAAAAAABlAAAAAAAAAAMCADrUIHRM3rjlJN62XzjLUJXTDQAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAQAAAADkhVuboDyZuBz9qkCLPGYF/jNmapt51Hcp74xNrumNVgAAAAFVU0QAAAAAAOSFW5ugPJm4HP2qQIs8ZgX+M2Zqm3nUdynvjE2u6Y1WAAAAAAvrwgAAAAAAAAAAAfwmKjcAAABAEV6Lzok4i4C1jJA3PVVARGx2+yfVw8odprnnnG0hqkUUwKnvVQcd59UJwbfzTG7oxR5DvxflV4aQ6RmZsIcmDQ==",
					).Return(horizonResponse, nil).Once()

					validParams.Add("memo_type", "hash")
					validParams.Add("memo", "02003AD420744CDEB8E524DEB65F38CB5095D30D000000000000000000000000")
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 200, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})
			})

			Convey("source account does not exist", func() {
				mockHorizon.On(
					"LoadAccount",
					"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
				).Return(horizon.AccountResponse{}, errors.New("Not found")).Once()

				Convey("it should return error", func() {
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))
					assert.Equal(t, 400, statusCode)
					expectedResponse := horizon.SubmitTransactionResponse{Error: horizon.PaymentSourceNotExist}
					assert.Equal(t, expectedResponse.Marshal(), []byte(responseString))
				})
			})

			Convey("transaction failed in horizon", func() {
				mockHorizon.On(
					"LoadAccount",
					"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
				).Return(
					horizon.AccountResponse{
						SequenceNumber: "100",
					},
					nil,
				).Once()

				horizonResponse := horizon.SubmitTransactionResponse{
					nil,
					&horizon.SubmitTransactionResponseError{
						Status: 400,
						Code:   "transaction_failed",
					},
					&horizon.SubmitTransactionResponseExtras{
						EnvelopeXdr: "envelope",
						ResultXdr:   "result",
					},
				}

				mockHorizon.On(
					"SubmitTransaction",
					mock.AnythingOfType("string"),
				).Return(horizonResponse, nil).Once()

				Convey("it should return error", func() {
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 400, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})
			})

			Convey("transaction success (native)", func() {
				validParams := url.Values{
					// GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ
					"source":      {"SDWLS4G3XCNIYPKXJWWGGJT6UDY63WV6PEFTWP7JZMQB4RE7EUJQN5XM"},
					"destination": {"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632"},
					"amount":      {"20"},
				}

				// Checking if destination exists
				mockHorizon.On(
					"LoadAccount",
					"GDSIKW43UA6JTOA47WVEBCZ4MYC74M3GNKNXTVDXFHXYYTNO5GGVN632",
				).Return(horizon.AccountResponse{}, nil).Once()

				// Loading sequence number
				mockHorizon.On(
					"LoadAccount",
					"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
				).Return(
					horizon.AccountResponse{
						SequenceNumber: "100",
					},
					nil,
				).Once()

				var ledger uint64
				ledger = 1988727
				horizonResponse := horizon.SubmitTransactionResponse{&ledger, nil, nil}

				mockHorizon.On(
					"SubmitTransaction",
					"AAAAAIu7VxM5f9eQ3va0bpvKprxnSHB4zyEnY4D/VzT8Jio3AAAAZAAAAAAAAABlAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAA5IVbm6A8mbgc/apAizxmBf4zZmqbedR3Ke+MTa7pjVYAAAAAAAAAAAvrwgAAAAAAAAAAAfwmKjcAAABAh3M6y9LXiWD0GB1KCkgNS5H1Lnyr1wS1BsfzoM1/v0muzobwNkJinV+RcWyC8VfeKqOjKBOANJnEusl+sHkcAg==",
				).Return(horizonResponse, nil).Once()

				Convey("it should return success", func() {
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 200, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})
			})

			Convey("transaction success (credit)", func() {
				mockHorizon.On(
					"LoadAccount",
					"GCF3WVYTHF75PEG6622G5G6KU26GOSDQPDHSCJ3DQD7VONH4EYVDOGKJ",
				).Return(
					horizon.AccountResponse{
						SequenceNumber: "100",
					},
					nil,
				).Once()

				var ledger uint64
				ledger = 1988727
				horizonResponse := horizon.SubmitTransactionResponse{&ledger, nil, nil}

				mockHorizon.On(
					"SubmitTransaction",
					"AAAAAIu7VxM5f9eQ3va0bpvKprxnSHB4zyEnY4D/VzT8Jio3AAAAZAAAAAAAAABlAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAA5IVbm6A8mbgc/apAizxmBf4zZmqbedR3Ke+MTa7pjVYAAAABVVNEAAAAAADkhVuboDyZuBz9qkCLPGYF/jNmapt51Hcp74xNrumNVgAAAAAL68IAAAAAAAAAAAH8Jio3AAAAQHbXpCBe/lDG5rWhwNpdH+DnrkYKONvMyPJDFik5mC/gcIL9xHx3FfB+u1Ik7N9gJxi8AlRRqXo+/yCyOoQQ3Ac=",
				).Return(horizonResponse, nil).Once()

				Convey("it should return success", func() {
					statusCode, response := getResponse(testServer, validParams)
					responseString := strings.TrimSpace(string(response))

					expectedResponse, err := json.MarshalIndent(horizonResponse, "", "  ")
					if err != nil {
						panic(err)
					}

					assert.Equal(t, 200, statusCode)
					assert.Equal(t, string(expectedResponse), responseString)
				})
			})
		})
	})
}