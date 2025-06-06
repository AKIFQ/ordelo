package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func sendResponse(ctx context.Context, w http.ResponseWriter, httpStatus int, messageMap *map[string]any, source slog.Attr) {
	ctx, span := Tracer.Start(ctx, "sendReponse")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(messageMap); err != nil {
		Logger.ErrorContext(ctx, "Error in encoding the message map", slog.Any("error", err), source)
		http.Error(w, "Oops!", http.StatusInternalServerError)
		return
	}
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "CreateUser")
	defer span.End()
	source := slog.String("source", "CreateUser")

	user := &Common{}
	if err := json.NewDecoder(r.Body).Decode(user); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse the request body to a user struct", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing Request body", source)
		return
	}

	Logger.InfoContext(ctx, "Validating user struct fields", source)
	switch {
	case user.Name == "":
		sendFailure(ctx, w, "Username is empty", source)
		return

	case user.Email == "":
		sendFailure(ctx, w, "Email is empty", source)
		return

	case user.PasswordHash == "":
		sendFailure(ctx, w, "Password is empty", source)
		return

	case user.Role == "":
		sendFailure(ctx, w, "role is empty", source)
		return
	}
	Logger.InfoContext(ctx, "Validated Successfully", source)

	userID, err := AuthService.CreateUser(ctx, user)
	if err != nil {
		sendResponse(ctx, w, http.StatusInternalServerError, &map[string]any{"success": false, "error": "Registration failed"}, source)
		return
	}

	okResponseMap := map[string]any{
		"status": true,
		"id":     userID.String(),
	}
	sendResponse(ctx, w, http.StatusCreated, &okResponseMap, source)
}

func UserLogin(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UserLogin")
	defer span.End()
	source := slog.String("source", "UserLogin")

	var req *Login
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse the request body to a Login struct", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing Request body", source)
		return
	}

	Logger.InfoContext(ctx, "Validating login struct fields", source)
	switch {
	case req.Email == "":
		sendFailure(ctx, w, "Email is empty", source)
		return

	case req.Password == "":
		sendFailure(ctx, w, "Password is empty", source)
		return

	case req.Role == "":
		sendFailure(ctx, w, "Role is empty", source)
		return
	}
	Logger.InfoContext(ctx, "Validated Successfully", source)

	id, accessToken, refreshToken, err := AuthService.Login(ctx, req)
	if err != nil {
		Logger.ErrorContext(ctx, "Error getting accessToken and refreshToken from auth service", slog.Any("error", err), source)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   60 * 60 * 24 * 7,
	})

	okResponseMap := map[string]any{
		"_id":          id.String(),
		"role":         req.Role,
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   "900",
	}

	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func CreateCarts(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "CreateCarts")
	defer span.End()
	source := slog.String("source", "CreateCarts")

	Logger.InfoContext(ctx, "Creating Carts to be added", source)
	req, err := decodeStruct[RequestCarts](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing carts request body", source)
		return
	}
	createCon(ctx, w, r, source, req.Carts)
}

func CreateUserOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "CreateUserOrders")
	defer span.End()
	source := slog.String("source", "CreateUserOrders")

	Logger.InfoContext(ctx, "Creating UserOrders to be added", source)
	req, err := decodeStruct[RequestUserOrders](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing userOrders request body", source)
		return
	}
	createCon(ctx, w, r, source, req.Orders)
}

func CreateVendorOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "CreateVendorOrders")
	defer span.End()
	source := slog.String("source", "CreateVendorOrders")

	Logger.InfoContext(ctx, "Creating VendorOrders to be added", source)
	req, err := decodeStruct[RequestVendorOrders](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing VendorOrders request body", source)
		return
	}
	createCon(ctx, w, r, source, req.Orders)
}

func VendorComparedItemsValue(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetItemsComparedValue")
	defer span.End()
	source := slog.String("source", "GetItemsComparedValue")

	Logger.InfoContext(ctx, "Getting the camparison request", source)
	req, err := decodeStruct[ReqIngArray](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing userOrders request body", source)
		return
	}
	Logger.InfoContext(ctx, "Decoded the ReqIng Struct successfully", source)

	res, err := Repos.Vendor.FindAllIngredients(ctx, req.Compare)
	if err != nil {
		if errors.Is(err, &NoItems{}) {
			Logger.ErrorContext(ctx, "No items found", slog.Any("error", err))
			sendFailure(ctx, w, "No items found", source)
			return
		}
		sendFailure(ctx, w, "Error in fetchig compared value", source)
		return
	}
	makeItemsOne(res)

	s, err := json.Marshal(res)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in marshalling to string", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in fetchig compared value", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"ids":     string(s),
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func CreateStores(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "CreateStores")
	defer span.End()
	source := slog.String("source", "CreateStores")

	Logger.InfoContext(ctx, "Creating stores to be added", source)
	req, err := decodeStruct[RequestStores](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing Stores request body", source)
		return
	}
	createCon(ctx, w, r, source, req.Stores)
}

func CreateRecipes(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "CreateRecipes")
	defer span.End()
	source := slog.String("source", "CreateRecipes")

	Logger.InfoContext(ctx, "Creating recipes to be added", source)
	req, err := decodeStruct[RequestRecipes](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing recipes request body", source)
		return
	}
	createCon(ctx, w, r, source, req.Recipes)
}

func AdminGetUsers(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminGetUsers")
	defer span.End()
	source := slog.String("source", "AdminGetUsers")

	Logger.InfoContext(ctx, "Admin retrieving all users", source)

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID", source)
		return
	}

	users, err := Repos.Admin.FindUsers(ctx, id)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to fetch users", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to fetch users", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"users":   users,
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)

}

func AdminGetVendors(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminGetVendors")
	defer span.End()
	source := slog.String("source", "AdminGetVendors")

	Logger.InfoContext(ctx, "Admin retrieving all vendors", source)

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID", source)
		return
	}

	vendors, err := Repos.Admin.FindVendors(ctx, id)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to fetch vendors", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to fetch vendors", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"vendors": vendors,
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func AdminGetStores(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminGetStores")
	defer span.End()
	source := slog.String("source", "AdminGetStores")

	Logger.InfoContext(ctx, "Admin retrieving all stores", source)

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID", source)
		return
	}

	vendors, err := Repos.Admin.FindStores(ctx, id)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to fetch stores", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to fetch stores", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"vendors": vendors,
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func AdminGetIngredients(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminGetIngredients")
	defer span.End()
	source := slog.String("source", "AdminGetIngredients")

	Logger.InfoContext(ctx, "Admin retrieving all ingredients", source)

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID", source)
		return
	}

	ingredients, err := Repos.Admin.FindIngredients(ctx, id)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to fetch ingredients", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to fetch ingredients", source)
		return
	}

	okResponseMap := map[string]any{
		"success":     true,
		"ingredients": ingredients,
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UpdateUser")
	defer span.End()
	source := slog.String("source", "UpdateUser")

	Logger.InfoContext(ctx, "Updating User information", source)

	var com *Common
	if err := json.NewDecoder(r.Body).Decode(&com); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse the request body to common struct", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing request body", source)
		return
	}

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get ID from context", source)
		return
	}

	com.ID = id.value
	role, ok := r.Context().Value(userRoleKey).(string)
	if !ok {
		sendFailure(ctx, w, "Unauthorized - missing role claim", source)
		return
	}

	switch role {
	case "user":
		if err := Repos.User.UpdateUser(ctx, com); err != nil {
			Logger.ErrorContext(ctx, "Failed to update user", slog.Any("error", err), source)
			sendFailure(ctx, w, "Failed to update user", source)
			return
		}
	case "vendor":
		if err := Repos.Vendor.UpdateVendor(ctx, com); err != nil {
			Logger.ErrorContext(ctx, "Failed to update vendor", slog.Any("error", err), source)
			sendFailure(ctx, w, "Failed to update vendor", source)
			return
		}
	case "admin":
		if err := Repos.Admin.UpdateAdmin(ctx, com); err != nil {
			Logger.ErrorContext(ctx, "Failed to update admin", slog.Any("error", err), source)
			sendFailure(ctx, w, "Failed to update admin", source)
			return
		}
	}

	okResponseMap := map[string]any{
		"success": true,
		"message": "User updated successfully",
	}

	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func AdminCreateIngredients(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminCreateIngredients")
	defer span.End()
	source := slog.String("source", "AdminCreateIngredients")

	Logger.InfoContext(ctx, "Creating ingredients", source)
	var req struct {
		Ingredients []*Ingredient `json:"ingredients"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse the request body", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing request body", source)
		return
	}

	if len(req.Ingredients) == 0 {
		Logger.ErrorContext(ctx, "No ingredients provided", source)
		sendFailure(ctx, w, "No ingredients provided", source)
		return
	}

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID from context", source)
		return
	}

	ids, err := Repos.Admin.CreateIngredients(ctx, id, req.Ingredients)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to create ingredients", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to create ingredients", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"ids":     getStringIDs(ids),
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func AdminUpdateIngredients(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminUpdateIngredients")
	defer span.End()
	source := slog.String("source", "AdminUpdateIngredients")

	Logger.InfoContext(ctx, "Updating ingredients", source)

	var req struct {
		Ingredients []*Ingredient `json:"ingredients"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse the request body", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing request body", source)
		return
	}

	if len(req.Ingredients) == 0 {
		Logger.ErrorContext(ctx, "No ingredients provided", source)
		sendFailure(ctx, w, "No ingredients provided", source)
		return
	}

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID from context", source)
		return
	}

	if err := Repos.Admin.UpdateIngredients(ctx, id, req.Ingredients); err != nil {
		Logger.ErrorContext(ctx, "Failed to update ingredients", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to update ingredients", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"message": "Ingredients updated successfully",
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func AdminDeleteIngredients(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminDeleteIngredients")
	defer span.End()
	source := slog.String("source", "AdminDeleteIngredients")

	Logger.InfoContext(ctx, "Deleting ingredients", source)

	var req struct {
		IngredientIDs []string `json:"ingredient_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse the request body", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing request body", source)
		return
	}

	if len(req.IngredientIDs) == 0 {
		Logger.ErrorContext(ctx, "No ingredient IDs provided", source)
		sendFailure(ctx, w, "No ingredient IDs provided", source)
		return
	}

	adminID, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get admin ID from context", source)
		return
	}

	ids := make([]*ID, len(req.IngredientIDs))
	for i, idStr := range req.IngredientIDs {
		id, err := NewID(ctx, idStr)
		if err != nil {
			Logger.ErrorContext(ctx, "Invalid ingredient ID format",
				slog.String("id", idStr), slog.Any("error", err), source)
			sendFailure(ctx, w, "Invalid ingredient ID format: "+idStr, source)
			return
		}
		ids[i] = &id
	}

	if err := Repos.Admin.DeleteIngredients(ctx, adminID, ids); err != nil {
		Logger.ErrorContext(ctx, "Failed to delete ingredients", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to delete ingredients", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"message": "Ingredients deleted successfully",
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func GetCarts(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetCarts")
	defer span.End()
	source := slog.String("source", "GetCarts")

	Logger.InfoContext(ctx, "Getting the Carts", source)
	var carts []*Cart
	getCon(ctx, w, r, carts, source)
}

func GetUserOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetUserOrders")
	defer span.End()
	source := slog.String("source", "GetUserOrders")

	Logger.InfoContext(ctx, "Getting the UserOrders", source)
	var userOrders []*UserOrder
	getCon(ctx, w, r, userOrders, source)
}

func GetVendorOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetVendorOrders")
	defer span.End()
	source := slog.String("source", "GetVendorOrders")

	Logger.InfoContext(ctx, "Getting the VendorOrders", source)
	var vendorOrders []*VendorOrder
	getCon(ctx, w, r, vendorOrders, source)
}

func GetStores(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetStores")
	defer span.End()
	source := slog.String("source", "GetStores")

	Logger.InfoContext(ctx, "Getting the Stores", source)
	var stores []*Store
	getCon(ctx, w, r, stores, source)
}

func GetItems(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetItems")
	defer span.End()
	source := slog.String("source", "GetItems")
	vid := r.PathValue("vid")
	if vid == "" {
		Logger.ErrorContext(ctx, "No vendorID provided in the path parms", source)
		sendFailure(ctx, w, "No vendorID provided in the path parms", source)
		return
	}

	sid := r.PathValue("sid")
	if sid == "" {
		Logger.ErrorContext(ctx, "Failed to fetch Admin ingredients", source)
		sendFailure(ctx, w, "Failed to fetch Admin ingredients", source)
		return
	}

	v_id, err := NewID(ctx, vid)
	if err != nil {
		sendFailure(ctx, w, "Unable to parse vendorID", source)
	}

	s_id, err := NewID(ctx, sid)
	if err != nil {
		sendFailure(ctx, w, "Unable to parse storeID", source)

	}

	Logger.InfoContext(ctx, "Getting the All items in store", slog.String("vendorID", v_id.String()),
		slog.String("storeID", s_id.String()), source)

	res, err := Repos.Vendor.FindVendorStore(ctx, v_id, s_id)
	if err != nil {
		if errors.Is(err, &NoItems{}) {
			Logger.ErrorContext(ctx, "No items found", slog.Any("error", err))
			sendFailure(ctx, w, "No items found", source)
			return
		}
		sendFailure(ctx, w, "Error in fetchig compared value", source)
		return
	}

	s, err := json.Marshal(res)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in marshalling to string", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in fetchig compared value", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"ids":     string(s),
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func GetUserAdminIngredients(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetUserAdminIngredients")
	defer span.End()

	sendIngredients(ctx, w, slog.String("source", "GetUserAdminIngredients"))
}

func GetVendorAdminIngredients(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetVendorAdminIngredients")
	defer span.End()

	sendIngredients(ctx, w, slog.String("source", "GetVendorAdminIngredients"))
}

func sendIngredients(ctx context.Context, w http.ResponseWriter, source slog.Attr) {
	Logger.InfoContext(ctx, "Getting the Admin Ingredients", source)
	res, err := getAllIngredients(ctx, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to fetch Admin ingredients", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to fetch Admin ingredients", source)
		return
	}

	b, err := json.Marshal(res)
	if err != nil {
		Logger.ErrorContext(ctx, "Failed to marshall the ingredients array to string", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to marshall the ingredients array to string", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"message": string(b),
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetUser")
	defer span.End()
	source := slog.String("source", "GetUser")

	Logger.InfoContext(ctx, "Getting the User", source)
	id, err := getID(ctx, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Unable to get the id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}
	user, err := Repos.User.FindUserByID(ctx, id)
	if err != nil {
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	s, err := json.Marshal(user)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in marshalling to string", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in fetching user", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"ids":     string(s),
	}

	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func GetRecipes(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "GetRecipes")
	defer span.End()
	source := slog.String("source", "GetRecipes")

	Logger.InfoContext(ctx, "Getting the Recipes", source)
	var recipes []*Recipe
	getCon(ctx, w, r, recipes, source)
}

func UpdateCarts(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UpdateCarts")
	defer span.End()
	source := slog.String("source", "UpdateCarts")

	Logger.InfoContext(ctx, "Updating Carts", source)
	req, err := decodeStruct[RequestCarts](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing carts request body", source)
		return
	}
	createCon(ctx, w, r, source, req.Carts)
}

func UpdateUserOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UpdateUserOrders")
	defer span.End()
	source := slog.String("source", "UpdateUserOrders")

	Logger.InfoContext(ctx, "Updating UserOrders", source)
	req, err := decodeStruct[RequestUserOrders](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing userOrders request body", source)
		return
	}
	updateCon(ctx, w, r, source, req.Orders)
}

func UpdateVendorOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UpdateVendorOrders")
	defer span.End()
	source := slog.String("source", "UpdateVendorOrders")

	Logger.InfoContext(ctx, "Updating VendorOrders", source)
	req, err := decodeStruct[RequestVendorOrders](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing VendorOrders request body", source)
		return
	}
	updateCon(ctx, w, r, source, req.Orders)
}

func UpdateStores(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UpdateStores")
	defer span.End()
	source := slog.String("source", "UpdateStores")

	Logger.InfoContext(ctx, "Updating stores ", source)
	req, err := decodeStruct[RequestStores](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing Stores request body", source)
		return
	}
	updateCon(ctx, w, r, source, req.Stores)
}

func AcceptUserOrder(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AcceptUserOrder")
	defer span.End()
	source := slog.String("source", "AcceptUserOrder")

	Logger.InfoContext(ctx, "Processing order status update", source)
	var req AcceptUserOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse request body", slog.Any("error", err), source)
		sendFailure(ctx, w, "Error in parsing request body", source)
		return
	}

	Logger.InfoContext(ctx, "Validating request fields", source)
	switch {
	case req.UserID == bson.NilObjectID:
		sendFailure(ctx, w, "User ID is required", source)
		return
	case req.OrderID == bson.NilObjectID:
		sendFailure(ctx, w, "Order ID is required", source)
		return
	case req.OrderStatus == "":
		sendFailure(ctx, w, "Order status is required", source)
		return
	}
	Logger.InfoContext(ctx, "Validated Successfully", source)

	vendorID, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Unable to get vendor ID from context", source)
		return
	}

	if err := Repos.Vendor.UpdateUserOrder(ctx, vendorID, &req); err != nil {
		sendFailure(ctx, w, "Failed to update order status", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
		"message": "Order status updated successfully",
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func UpdateRecipes(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "UpdateRecipes")
	defer span.End()
	source := slog.String("source", "UpdateRecipes")

	Logger.InfoContext(ctx, "Updating recipes", source)
	req, err := decodeStruct[RequestRecipes](ctx, r.Body, source)
	if err != nil {
		sendFailure(ctx, w, "Error in parsing recipes request body", source)
		return
	}
	updateCon(ctx, w, r, source, req.Recipes)
}

func DeleteCarts(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteCarts")
	defer span.End()
	source := slog.String("source", "DeleteCarts")

	Logger.InfoContext(ctx, "Deleting carts ", source)
	id, err := getID(r.Context(), source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	ids, err := decodeToIDS(ctx, r.Body, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting ids from body", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	if err := Repos.User.DeleteCarts(ctx, id, ids); err != nil {
		Logger.ErrorContext(ctx, "Error in deleting carts", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "carts deleted successfully"}, source)
}

func DeleteCartItems(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteCartItems")
	defer span.End()
	source := slog.String("source", "DeleteCartItems")

	Logger.InfoContext(ctx, "Deleting cart items ", source)
	id, err := getID(r.Context(), source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	conId, Ids, err := getDeleteIds(ctx, r.Body, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting con ids and items ids array from body", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	if err := Repos.User.DeleteCartItems(ctx, id, conId, Ids); err != nil {
		Logger.ErrorContext(ctx, "Error in deleting cart items", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "cart items deleted successfully"}, source)
}

func DeleteStores(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteStores")
	defer span.End()
	source := slog.String("source", "DeleteStores")

	Logger.InfoContext(ctx, "Deleting stores ", source)
	id, err := getID(r.Context(), source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	ids, err := decodeToIDS(ctx, r.Body, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting ids from body", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	if err := Repos.Vendor.DeleteStores(ctx, id, ids); err != nil {
		Logger.ErrorContext(ctx, "Error in delete stores", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "stores deleted successfully"}, source)

}

func DeleteStoreItems(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteStoreItems")
	defer span.End()
	source := slog.String("source", "DeleteStoreItems")

	Logger.InfoContext(ctx, "Deleting Store items ", source)
	id, err := getID(r.Context(), source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	conId, Ids, err := getDeleteIds(ctx, r.Body, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting con ids and items ids array from body", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	if err := Repos.Vendor.DeleteStoreItems(ctx, id, conId, Ids); err != nil {
		Logger.ErrorContext(ctx, "Error in deleting store items", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "store items deleted successfully"},
		source)
}

func DeleteRecipes(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteRecipes")
	defer span.End()
	source := slog.String("source", "DeleteRecipes")

	Logger.InfoContext(ctx, "Deleting Recipes ", source)
	id, err := getID(r.Context(), source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	ids, err := decodeToIDS(ctx, r.Body, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting ids from body", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	if err := Repos.User.DeleteRecipes(ctx, id, ids); err != nil {
		Logger.ErrorContext(ctx, "Error in delete recipes", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "recipes deleted successfully"}, source)
}

func DeleteRecipeItems(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteRecipeItems")
	defer span.End()
	source := slog.String("source", "DeleteRecipeItems")

	Logger.InfoContext(ctx, "Deleting Recipe items ", source)
	id, err := getID(r.Context(), source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting id from the token", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	conId, Ids, err := getDeleteIds(ctx, r.Body, source)
	if err != nil {
		Logger.ErrorContext(ctx, "Error in getting con ids and items ids array from body", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	if err := Repos.User.DeleteRecipeItems(ctx, id, conId, Ids); err != nil {
		Logger.ErrorContext(ctx, "Error in deleting recipe items", slog.Any("error", err), source)
		sendFailure(ctx, w, err.Error(), source)
		return
	}

	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "recipe items deleted successfully"},
		source)
}

func DeleteAdmin(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteAdmin")
	defer span.End()
	source := slog.String("source", "DeleteAdmin")

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Oops", source)
		return
	}

	if err = Repos.Admin.Delete(ctx, id); err != nil {
		sendFailure(ctx, w, err.Error(), source)
		return
	}
	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "Admin deleted successfully"}, source)

}

func AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminDeleteUser")
	defer span.End()
	source := slog.String("source", "AdminDeleteUser")

	id, err := NewID(ctx, r.PathValue("id"))
	if err != nil {
		Logger.Error("Unable to convert id req to ID", slog.Any("error", err), source)
		sendFailure(ctx, w, "Invalid id", source)
		return
	}

	if err = Repos.Admin.DeleteUser(ctx, id); err != nil {
		sendFailure(ctx, w, err.Error(), source)
		return
	}
	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "User deleted successfully"}, source)

}

func AdminDeleteVendor(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "AdminDeleteVendor")
	defer span.End()
	source := slog.String("source", "AdminDeleteVendor")

	id, err := NewID(ctx, r.PathValue("id"))
	if err != nil {
		Logger.Error("Unable to convert id req to ID", slog.Any("error", err), source)
		sendFailure(ctx, w, "Invalid id", source)
		return
	}

	if err = Repos.Admin.DeleteVendor(ctx, id); err != nil {
		sendFailure(ctx, w, err.Error(), source)
		return
	}
	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "Vendor deleted successfully"}, source)
}

func DeleteVendor(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteVendor")
	defer span.End()
	source := slog.String("source", "DeleteVendor")

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Oops", source)
		return
	}

	if err = Repos.Vendor.DeleteVendor(ctx, id); err != nil {
		sendFailure(ctx, w, err.Error(), source)
		return
	}
	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "Vendor deleted successfully"}, source)

}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx, span := Tracer.Start(r.Context(), "DeleteUser")
	defer span.End()
	source := slog.String("source", "DeleteUser")

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Oops", source)
		return
	}

	if err = Repos.User.DeleteUser(ctx, id); err != nil {
		sendFailure(ctx, w, err.Error(), source)
		return
	}
	sendResponse(ctx, w, http.StatusOK, &map[string]any{"success": true, "message": "User deleted successfully"}, source)
}

func sendFailure(ctx context.Context, w http.ResponseWriter, err string, source slog.Attr) {
	ctx, span := Tracer.Start(ctx, "SendFailure")
	defer span.End()

	errorResponseMap := map[string]any{
		"success": false,
		"error":   err,
	}
	sendResponse(ctx, w, http.StatusBadRequest, &errorResponseMap, source)
}

func createCon[C containers](ctx context.Context, w http.ResponseWriter, r *http.Request, source slog.Attr, con C) {
	var err error
	var ids []*ID

	defer func() {
		if err != nil {
			Logger.ErrorContext(ctx, "Failed to create containers", slog.Any("error", err), source)
			sendFailure(ctx, w, "Failed to create containers", source)
		}
	}()

	id, err := getID(r.Context(), source)
	if err != nil {
		return
	}

	if len(con) == 0 {
		Logger.ErrorContext(ctx, "No items provided", source)
		err = errors.New("no items provided")
		return
	}

	switch c := any(con).(type) {
	case []*Cart:
		ids, err = Repos.User.CreateCarts(ctx, id, c)
	case []*Recipe:
		ids, err = Repos.User.CreateRecipes(ctx, id, c)
	case []*UserOrder:
		if ids, err = Repos.User.CreateUserOrders(ctx, id, c); err != nil {
			Logger.ErrorContext(ctx, "Error in user persistence", slog.Any("error", err), source)
			return
		}

		vg, vi := make(map[bson.ObjectID][]*VendorOrder), make(map[bson.ObjectID][]*ID)
		for _, v := range c {
			vg[v.VendorID] = append(vg[v.VendorID], &VendorOrder{Order: v.Order, UserID: id.value})
		}

		for v, o := range vg {
			oids, ordErr := Repos.Vendor.CreateVendorOrders(ctx, ID{v}, o)
			if ordErr != nil {
				Logger.ErrorContext(ctx, "Error in Vendor persistence Rolling back created user and vendor orders",
					slog.Any("error", ordErr), source)
				err = errors.Join(err, ordErr)

				if userErr := Repos.User.DeleteUserOrders(ctx, id, ids); userErr != nil {
					Logger.ErrorContext(ctx, "Error in deleting newly created User orders", slog.Any("error", userErr), source)
					err = errors.Join(err, userErr)
				}

				for vid, oids := range vi {
					if venErr := Repos.Vendor.DeleteVendorOrders(ctx, ID{vid}, oids); venErr != nil {
						Logger.ErrorContext(ctx, "Error in deleting newly created Vendor orders", slog.Any("error", venErr), source)
						err = errors.Join(err, venErr)
					}
				}
				break
			}
			vi[v] = oids
		}
	case []*Store:
		ids, err = Repos.Vendor.CreateStores(ctx, id, c)
	case []*VendorOrder:
		ids, err = Repos.Vendor.CreateVendorOrders(ctx, id, c)
	default:
		Logger.ErrorContext(ctx, "Unable to get the id String fromn context", source)
		err = errors.New("unknown type")
		return
	}
	if err != nil {
		return
	}
	okResponseMap := map[string]any{
		"success": true,
		"ids":     getStringIDs(ids),
	}
	sendResponse(ctx, w, http.StatusCreated, &okResponseMap, source)
}

func updateCon[C containers](ctx context.Context, w http.ResponseWriter, r *http.Request, source slog.Attr, con C) {
	var err error

	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Oops", source)
		return
	}

	if len(con) == 0 {
		Logger.ErrorContext(ctx, "No items provided", source)
		sendFailure(ctx, w, "No items provided", source)
		return
	}

	switch c := any(con).(type) {
	case []*Cart:
		err = Repos.User.UpdateCarts(ctx, id, c)
	case []*Recipe:
		err = Repos.User.UpdateRecipes(ctx, id, c)
	case []*UserOrder:
		err = Repos.User.UpdateUserOrders(ctx, id, c)
	case []*Store:
		err = Repos.Vendor.UpdateStores(ctx, id, c)
	case []*VendorOrder:
		err = Repos.Vendor.UpdateVendorOrders(ctx, id, c)
	default:
		Logger.ErrorContext(ctx, "Unable to get the id String fromn context", source)
		sendFailure(ctx, w, "unknown type", source)
		return
	}

	if err != nil {
		Logger.ErrorContext(ctx, "Failed to update containers", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to update containers", source)
		return
	}

	okResponseMap := map[string]any{"success": true}
	sendResponse(ctx, w, http.StatusCreated, &okResponseMap, source)
}

func getCon[c containers](ctx context.Context, w http.ResponseWriter, r *http.Request, t c, source slog.Attr) {
	var err error
	id, err := getID(r.Context(), source)
	if err != nil {
		sendFailure(ctx, w, "Oops", source)
		return
	}

	okResponseMap := map[string]any{
		"success": true,
	}

	switch a := any(t).(type) {
	case []*Recipe:
		a, err = Repos.User.FindRecipes(ctx, id)
		okResponseMap["value"] = a
	case []*Cart:
		a, err = Repos.User.FindCarts(ctx, id)
		okResponseMap["value"] = a
	case []*UserOrder:
		a, err = Repos.User.FindUserOrders(ctx, id)
		okResponseMap["value"] = a
	case []*Store:
		a, err = Repos.Vendor.FindStores(ctx, id)
		okResponseMap["value"] = a
	case []*VendorOrder:
		a, err = Repos.Vendor.FindVendorOrders(ctx, id)
		okResponseMap["value"] = a
	default:
		Logger.ErrorContext(ctx, "Unknown get container constant", source)
		sendFailure(ctx, w, "unknown get container constant", source)
		return
	}

	if err != nil {
		Logger.ErrorContext(ctx, "Failed to fetch containers", slog.Any("error", err), source)
		sendFailure(ctx, w, "Failed to fetch containers", source)
		return
	}
	sendResponse(ctx, w, http.StatusOK, &okResponseMap, source)
}

func decodeStruct[req ComConReq](ctx context.Context, r io.Reader, source slog.Attr) (v *req, err error) {
	Logger.InfoContext(ctx, "Decode the body to struct", source)
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse request body", slog.Any("error", err), source)
		return nil, err
	}
	Logger.InfoContext(ctx, "Decoded Successfully", source)
	return
}

func decodeToIDS(ctx context.Context, r io.Reader, source slog.Attr) (ids []*ID, err error) {
	Logger.InfoContext(ctx, "Decode the body to arrays of *IDs", source)
	if err = json.NewDecoder(r).Decode(&ids); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse request body", slog.Any("error", err), source)
		return
	}
	Logger.InfoContext(ctx, "Decoded Successfully", source)
	return
}

func getDeleteIds(ctx context.Context, r io.Reader, source slog.Attr) (ID, []bson.ObjectID, error) {
	Logger.InfoContext(ctx, "Decode the body to arrays of items IDs and container id", source)
	var req struct {
		Id  ID              `json:"id"`
		Ids []bson.ObjectID `json:"ids"`
	}

	if err := json.NewDecoder(r).Decode(&req); err != nil {
		Logger.ErrorContext(ctx, "Unable to parse request body", slog.Any("error", err), source)
		return req.Id, req.Ids, err
	}

	Logger.InfoContext(ctx, "Decoded Successfully", source)
	return req.Id, req.Ids, nil
}

func getID(ctx context.Context, source slog.Attr) (ID, error) {
	v, ok := ctx.Value(userIDKey).(string)
	if !ok {
		Logger.ErrorContext(ctx, "Unable to get the id String fromn context", source)
		return ID{bson.NilObjectID}, errors.New("unable to cast to string from ctx")
	}
	return NewID(ctx, v)
}

func makeItemsOne(res []*ResIng) {
	for _, v := range res {
		for _, s := range v.Stores {
			for _, i := range s.Items {
				i.Quantity = 1
			}
		}
	}
}
