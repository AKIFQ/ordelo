package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	log.Printf("Starting the Ordelo go backend\n")
	if err := run(); err != nil {
		log.Fatalf("Error fatal error in server -> \n%v\n", err)
	}
	log.Printf("Sever shutdown successfull with no errors\n")
}

func run() (err error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	log.Printf("Initing resources Otel, Mongo and Redis\n")
	otelShutDown, err := initOtelSDK(ctx)
	if err != nil {
		log.Printf("Error in initing otel -> %v\n", err)
		return
	}

	mongoShutDown, err := initDB(ctx)
	if err != nil {
		log.Printf("Error in initing mongoDB -> %v\n", err)
		return
	}

	redisShutDown, err := initRedis(ctx)
	if err != nil {
		log.Printf("Error in initing redis -> %v\n", err)
		return
	}

	log.Printf("Inited Successfully\n")
	defer func() {
		log.Printf("Cleaning up resources\n")
		err = errors.Join(err, mongoShutDown(context.Background()))
		err = errors.Join(err, redisShutDown(context.Background()))
		err = errors.Join(err, otelShutDown(context.Background()))
	}()

	log.Printf("Initing cached repositories and auth service\n")
	if err = InitCachedMongoRepositories(ctx, RedisClient, MongoClient, 15*time.Minute); err != nil {
		log.Printf("Error in initing cached repositories -> %v\n", err)
		return
	}
	if err = InitAuthService(ctx, Repos, RedisClient, 15*time.Hour, 7*24*time.Hour); err != nil {
		log.Printf("Error in initing auth service -> %v\n", err)
		return
	}

	log.Printf("Inited Successfully\n")

	log.Printf("Starting Server\n")
	port := os.Getenv("PORT")
	if port == "" {
		err = errors.New("env varible PORT is empty")
		return
	}
	srv := &http.Server{
		Addr:         ":" + port,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}

	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()
	log.Printf("Server started on port -> %s\n", port)

	select {
	case err = <-srvErr:
		return
	case <-ctx.Done():
		stop()
	}

	err = srv.Shutdown(context.Background())
	return
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	handleFunc := func(pattern string, hand http.Handler) {
		handler := otelhttp.WithRouteTag(pattern, hand)
		mux.Handle(pattern, handler)
	}

	mid := AuthService.JWTAuthMiddleware()
	admin := RoleAuthMiddleware("admin")
	vendor := RoleAuthMiddleware("vendor")
	user := RoleAuthMiddleware("user")

	//-------------Common-To-All-----------------------------
	handleFunc("POST /register", http.HandlerFunc(CreateUser))
	handleFunc("POST /login", http.HandlerFunc(UserLogin))
	//-------------------------------------------------------
	//
	//-------------Admin-Specific-----------------------------
	handleFunc("POST /admin/ingredients", mid(admin(http.HandlerFunc(AdminCreateIngredients))))

	handleFunc("GET /admin/users", mid(admin(http.HandlerFunc(AdminGetUsers))))
	handleFunc("GET /admin/vendors", mid(admin(http.HandlerFunc(AdminGetVendors))))
	handleFunc("GET /admin/stores", mid(admin(http.HandlerFunc(AdminGetStores))))
	handleFunc("GET /admin/ingredients", mid(admin(http.HandlerFunc(AdminGetIngredients))))

	handleFunc("PUT /admin", mid(admin(http.HandlerFunc(UpdateUser))))
	handleFunc("PUT /admin/ingredients", mid(admin(http.HandlerFunc(AdminUpdateIngredients))))

	handleFunc("DELETE /admin", mid(admin(http.HandlerFunc(DeleteAdmin))))
	handleFunc("DELETE /admin/user/{id}", mid(admin(http.HandlerFunc(AdminDeleteUser))))
	handleFunc("DELETE /admin/vendor/{id}", mid(admin(http.HandlerFunc(AdminDeleteVendor))))
	handleFunc("DELETE /admin/ingredients", mid(admin(http.HandlerFunc(AdminDeleteIngredients))))
	//--------------------------------------------------------
	//
	//-------------Vendor-Specific-----------------------------
	handleFunc("POST /vendor/stores", mid(vendor(http.HandlerFunc(CreateStores))))
	handleFunc("POST /vendor/orders", mid(vendor(http.HandlerFunc(CreateVendorOrders))))

	handleFunc("GET /vendor/stores", mid(vendor(http.HandlerFunc(GetStores))))
	handleFunc("GET /vendor/orders", mid(vendor(http.HandlerFunc(GetVendorOrders))))
	handleFunc("GET /vendor/ingredients", mid(vendor(http.HandlerFunc(GetVendorAdminIngredients))))

	handleFunc("PUT /vendor/stores", mid(vendor(http.HandlerFunc(UpdateStores))))
	handleFunc("PUT /vendor/userorder/accept", mid(vendor(http.HandlerFunc(AcceptUserOrder))))
	handleFunc("PUT /vendor/orders", mid(vendor(http.HandlerFunc(UpdateVendorOrders))))
	handleFunc("PUT /vendor/order/{id}", mid(vendor(http.HandlerFunc(UpdateVendorOrders))))
	handleFunc("PUT /vendor", mid(vendor(http.HandlerFunc(UpdateUser))))

	handleFunc("DELETE /vendor/stores", mid(vendor(http.HandlerFunc(DeleteStores))))
	handleFunc("DELETE /vendor/store/items/", mid(vendor(http.HandlerFunc(DeleteStoreItems))))
	handleFunc("DELETE /vendor", mid(vendor(http.HandlerFunc(DeleteVendor))))
	//---------------------------------------------------------
	//
	//-------------User-Specific-------------------------------
	handleFunc("POST /user/recipes", mid(user(http.HandlerFunc(CreateRecipes))))
	handleFunc("POST /user/carts", mid(user(http.HandlerFunc(CreateCarts))))
	handleFunc("POST /user/orders", mid(user(http.HandlerFunc(CreateUserOrders))))
	handleFunc("POST /user/items/compare", mid(user(http.HandlerFunc(VendorComparedItemsValue))))

	handleFunc("GET /user", mid(user(http.HandlerFunc(GetUser))))
	handleFunc("GET /user/recipes", mid(user(http.HandlerFunc(GetRecipes))))
	handleFunc("GET /user/carts", mid(user(http.HandlerFunc(GetCarts))))
	handleFunc("GET /user/orders", mid(user(http.HandlerFunc(GetUserOrders))))
	handleFunc("GET /user/ingredients", mid(user(http.HandlerFunc(GetUserAdminIngredients))))
	handleFunc("GET /vendor/{vid}/store/{sid}/items", mid(user(http.HandlerFunc(GetItems))))

	handleFunc("PUT /user", mid(user(http.HandlerFunc(UpdateUser))))
	handleFunc("PUT /user/recipes", mid(user(http.HandlerFunc(UpdateRecipes))))
	handleFunc("PUT /user/carts", mid(user(http.HandlerFunc(UpdateCarts))))
	handleFunc("PUT /user/orders", mid(user(http.HandlerFunc(UpdateUserOrders))))

	handleFunc("DELETE /user/recipes", mid(user(http.HandlerFunc(DeleteRecipes))))
	handleFunc("DELETE /user/recipe/items", mid(user(http.HandlerFunc(DeleteRecipeItems))))
	handleFunc("DELETE /user/carts", mid(user(http.HandlerFunc(DeleteCarts))))
	handleFunc("DELETE /user/cart/items", mid(user(http.HandlerFunc(DeleteCartItems))))
	handleFunc("DELETE /user", mid(user(http.HandlerFunc(DeleteUser))))

	return CORSMiddleware(otelhttp.NewHandler(mux, "/"))
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "3600")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
