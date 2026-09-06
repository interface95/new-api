package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type topUpUserListResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int              `json:"total"`
		Items    []map[string]any `json:"items"`
	} `json:"data"`
}

func requestTopUpUserList(t *testing.T, handler gin.HandlerFunc, query string) topUpUserListResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup"+query, nil)
	ctx.Set("id", 101)
	handler(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response topUpUserListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestTopUpUserDetails(t *testing.T) {
	previousGinMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousGinMode) })
	gin.SetMode(gin.TestMode)
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var dialector gorm.Dialector
			dbType := common.DatabaseType(dialect)
			switch dialect {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TOPUP_TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TOPUP_TEST_MYSQL_DSN is not configured")
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TOPUP_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TOPUP_TEST_POSTGRES_DSN is not configured")
				}
				dialector = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			db, err := gorm.Open(dialector, &gorm.Config{})
			require.NoError(t, err)
			connection, err := db.DB()
			require.NoError(t, err)
			connection.SetMaxOpenConns(1)
			previousDB, previousLogDB := model.DB, model.LOG_DB
			previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
			previousRedis := common.RedisEnabled
			model.DB, model.LOG_DB = db, db
			common.SetDatabaseTypes(dbType, dbType)
			common.RedisEnabled = false
			t.Cleanup(func() {
				model.DB, model.LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMainType, previousLogType)
				common.RedisEnabled = previousRedis
				require.NoError(t, db.Migrator().DropTable(&model.TopUp{}, &model.User{}))
				require.NoError(t, connection.Close())
			})
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}))
			privateToken := "private-topup-test-token"
			users := []model.User{
				{Id: 101, Username: "alice-login", DisplayName: "Alice Display", Password: "test-only", Email: "private-topup@example.com", AccessToken: &privateToken, AffCode: "alice", Quota: 5_000_000_000, UsedQuota: 2_000_000_000, Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
				{Id: 102, Username: "bob-login", Password: "unused", AffCode: "bob", Quota: 0, UsedQuota: 700},
				{Id: 103, Username: "soft-deleted", Password: "unused", AffCode: "soft", Quota: 80, UsedQuota: 20},
				{Id: 104, Username: "hard-deleted", Password: "unused", AffCode: "hard", Quota: 10},
			}
			require.NoError(t, db.Create(&users).Error)
			require.NoError(t, db.Delete(&model.User{}, 103).Error)
			require.NoError(t, db.Unscoped().Delete(&model.User{}, 104).Error)
			now := time.Now().Unix()
			orders := []model.TopUp{
				{Id: 1, UserId: 101, TradeNo: "ALICE-OLD", CreateTime: now - 31*24*60*60, Amount: 1, Status: common.TopUpStatusSuccess},
				{Id: 2, UserId: 101, TradeNo: "MATCH-ALICE", CreateTime: now, Amount: 2, Status: common.TopUpStatusPending},
				{Id: 3, UserId: 102, TradeNo: "MATCH-BOB", CreateTime: now, Amount: 3, Status: common.TopUpStatusSuccess},
				{Id: 4, UserId: 103, TradeNo: "SOFT-DELETED", CreateTime: now, Amount: 4, Status: common.TopUpStatusSuccess},
				{Id: 5, UserId: 104, TradeNo: "HARD-DELETED", CreateTime: now, Amount: 5, Status: common.TopUpStatusSuccess},
				{Id: 6, UserId: 999, TradeNo: "MISSING-USER", CreateTime: now, Amount: 6, Status: common.TopUpStatusFailed},
			}
			require.NoError(t, db.Create(&orders).Error)
			userQueries := 0
			failUserQuery := false
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:topup_users", func(tx *gorm.DB) {
				if tx.Statement.Table == "users" {
					userQueries++
					if failUserQuery {
						tx.AddError(errors.New("topup user lookup unavailable"))
					}
				}
			}))
			alice := map[string]any{"username": "alice-login", "quota": float64(5_000_000_000), "used_quota": float64(2_000_000_000)}
			bob := map[string]any{"username": "bob-login", "quota": float64(0), "used_quota": float64(700)}
			softDeleted := map[string]any{"username": "soft-deleted", "quota": float64(80), "used_quota": float64(20)}
			for _, test := range []struct {
				name, query                    string
				ids                            []int
				total, page, pageSize, queries int
				users                          []any
			}{
				{"all with deleted users", "?page_size=100", []int{6, 5, 4, 3, 2, 1}, 6, 1, 100, 1, []any{nil, nil, softDeleted, bob, alice, alice}},
				{"page preserves repeated user and old order", "?p=3&page_size=2", []int{2, 1}, 6, 3, 2, 1, []any{alice, alice}},
				{"order search", "?keyword=MATCH%25&page_size=10", []int{3, 2}, 2, 1, 10, 1, []any{bob, alice}},
				{"search page", "?keyword=MATCH%25&p=2&page_size=1", []int{2}, 2, 2, 1, 1, []any{alice}},
				{"empty page", "?p=10&page_size=10", nil, 6, 10, 10, 0, nil},
			} {
				t.Run(test.name, func(t *testing.T) {
					userQueries = 0
					response := requestTopUpUserList(t, GetAllTopUps, test.query)
					require.True(t, response.Success)
					assert.Equal(t, test.total, response.Data.Total)
					assert.Equal(t, test.page, response.Data.Page)
					assert.Equal(t, test.pageSize, response.Data.PageSize)
					require.Len(t, response.Data.Items, len(test.ids))
					for i, item := range response.Data.Items {
						assert.Equal(t, float64(test.ids[i]), item["id"])
						assert.Equal(t, float64(orders[test.ids[i]-1].UserId), item["user_id"])
						assert.Equal(t, orders[test.ids[i]-1].TradeNo, item["trade_no"])
						assert.Equal(t, float64(orders[test.ids[i]-1].Amount), item["amount"])
						require.Contains(t, item, "user")
						assert.Equal(t, test.users[i], item["user"])
						assert.NotContains(t, item, "password")
						assert.NotContains(t, item, "access_token")
					}
					assert.Equal(t, test.queries, userQueries, "user lookups must remain bounded per page")
				})
			}
			t.Run("user lookup failure is an API failure", func(t *testing.T) {
				failUserQuery = true
				t.Cleanup(func() { failUserQuery = false })
				response := requestTopUpUserList(t, GetAllTopUps, "?keyword=MATCH%25")
				assert.False(t, response.Success)
				assert.Empty(t, response.Data.Items)
			})
			t.Run("self endpoint retains ownership window and original fields", func(t *testing.T) {
				userQueries = 0
				response := requestTopUpUserList(t, GetUserTopUps, "?user_id=102&page_size=100")
				require.True(t, response.Success)
				assert.Equal(t, 1, response.Data.Total)
				require.Len(t, response.Data.Items, 1)
				assert.Equal(t, float64(2), response.Data.Items[0]["id"])
				assert.NotContains(t, response.Data.Items[0], "user")
				assert.NotContains(t, response.Data.Items[0], "username")
				assert.NotContains(t, response.Data.Items[0], "quota")
				assert.NotContains(t, response.Data.Items[0], "used_quota")
				assert.Zero(t, userQueries)
				searched := requestTopUpUserList(t, GetUserTopUps, "?keyword=MATCH-BOB&user_id=102")
				require.True(t, searched.Success)
				assert.Zero(t, searched.Data.Total)
				assert.Empty(t, searched.Data.Items)
			})
			t.Run("ordinary user cannot access admin list", func(t *testing.T) {
				router := gin.New()
				router.GET("/api/user/topup", middleware.AdminAuth(), GetAllTopUps)
				request := httptest.NewRequest(http.MethodGet, "/api/user/topup", nil)
				request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", privateToken))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				assert.Equal(t, http.StatusForbidden, recorder.Code)
				assert.NotContains(t, recorder.Body.String(), "alice-login")
			})
		})
	}
}
