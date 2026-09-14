package main

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"

	"loyalty-service/internal/handler"
	"loyalty-service/internal/jwt"
	appMw "loyalty-service/internal/middleware"
	"loyalty-service/internal/service"
)

// setupRoutes настраивает все API-маршруты и возвращает настроенный роутер.
func setupRoutes(
	db *sql.DB,
	authService *service.AuthService,
	accrualSystemURL string,
) http.Handler {
	pointsChecker := service.NewPointsCalcClient(accrualSystemURL)
	orderServiceWithPoints := service.NewOrderService(db, pointsChecker)

	authHandler := handler.NewAuthHandler(authService)
	orderHandler := handler.NewOrderHandler(orderServiceWithPoints, authService)
	balanceHandler := handler.NewBalanceHandler(authService, orderServiceWithPoints)

	r := chi.NewRouter()
	r.Use(recoveryWrapper)

	// Маршруты аутентификации (публичные)
	r.Post("/api/user/register", authHandler.Register)
	r.Post("/api/user/login", authHandler.Login)
	r.Post("/api/user/refresh", authHandler.Refresh)
	r.Post("/api/user/logout", authHandler.Logout)

	// Защищённые маршруты
	userRouter := chi.NewRouter()
	userRouter.Use(jwt.AuthMiddleware)
	userRouter.Post("/orders", orderHandler.Submit)
	userRouter.Get("/orders", orderHandler.List)
	userRouter.Get("/balance", balanceHandler.Balance)
	userRouter.Post("/balance/withdraw", balanceHandler.Withdraw)
	userRouter.Get("/withdrawals", balanceHandler.Withdrawals)
	r.Mount("/", userRouter)

	// Публичный endpoint для расчёта
	r.Get("/api/orders/{number}", orderHandler.Calculation)

	return r
}

// recoveryWrapper оборачивает обработку паник и возвращает JSON-ошибку 500.
func recoveryWrapper(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("panic", err).Error("handler panic recovered")
				appMw.WriteError(w, http.StatusInternalServerError, appMw.ServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
