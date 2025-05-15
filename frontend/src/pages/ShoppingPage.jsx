import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import CartPanel from "../components/CartPanel";
import SelectedIngredientsPanel from "../components/SelectedIngredientsPanel";
import VendorDiscovery from "../components/VendorDiscovery";
import { useShoppingContext } from "../context/ShoppingContext";
import { calculateDistance } from "../components/distance.js";

function ShoppingPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const {
    showIngredientsPanel,
    showCartPanel,
    carts,
    ingredientsPanelRef,
    cartPanelRef,
    setVendors
  } = useShoppingContext();

  const [pageError, setPageError] = useState("");

  useEffect(() => {
    document.title = "Shopping | Ordelo";
    setPageError("");

    if (location.state?.stores && location.state?.userLocation) {
      const { stores: rawStores, userLocation } = location.state;

      if (!userLocation || userLocation.lat == null || userLocation.lng == null) {
        setPageError("Your location could not be determined. Distances will not be shown.");
      }

      try {
        const transformedVendors = rawStores.map(vendor => {
          if (!vendor?.stores) return null;

          return {
            id: vendor.vendor_id,
            name: vendor.name || "Unnamed Vendor",
            stores: vendor.stores.map(store => {
              const coords = store.location?.coordinates || [];
              const storeLat = store.location.coordinates[1];
              const storeLng = store.location.coordinates[0];
              
              let distanceInMiles = null;
              if (userLocation && userLocation.lat != null && userLocation.lng != null && storeLat != null && storeLng != null) {
                 distanceInMiles = calculateDistance(userLocation.lat, userLocation.lng, storeLat, storeLng);
              }

              return {
                // ... (other store properties) ...
                id: store.store_id,
                name: store.name || "Unnamed Store",
                storeType: store.store_type,
                location: store.location,
                // Store distance in miles, rounded to 1 decimal place
                distance: distanceInMiles !== null ? parseFloat(distanceInMiles.toFixed(1)) : null, 
                matchingItems: (store.items || []).length,
                totalItems: (store.items || []).length, 
                totalPrice: (store.items || []).reduce((sum, item) => sum + ((item.price || 0) * (item.quantity || 0)), 0),
                availableItems: (store.items || []).map(item => ({
                  id: item.ingredient_id,
                  name: item.name,
                  price: item.price || 0,
                  quantity: item.quantity || 0, 
                  unit: item.unit,
                  unitQuantity: item.unit_quantity
                }))
              };
            }).filter(Boolean),
          };
        }).filter(Boolean);

        setVendors(transformedVendors);
      } catch (err) {
        console.error("Transformation error:", err);
        setPageError("Could not load vendor data. Please try again.");
      }

    } else {
      setPageError("Required data not found. Redirecting...");
      setTimeout(() => navigate('/shopping-list'), 1500);
    }
  }, [location.state, navigate, setVendors]);

  const hasActiveCarts = Object.keys(carts).length > 0;

  return (
    <div className="shopping-page">
      {pageError && (
        <div className="error-message" style={{ color: 'red', textAlign: 'center', padding: '1rem' }}>
          {pageError}
        </div>
      )}

      <div className={`shopping-layout ${showIngredientsPanel ? 'show-ingredients' : ''} ${hasActiveCarts && showCartPanel ? 'show-cart' : ''}`}>
        <div ref={ingredientsPanelRef} className={`ingredients-panel-container ${showIngredientsPanel ? 'expanded' : ''}`}>
          <SelectedIngredientsPanel />
        </div>

        <div className="vendor-discovery-container">
          <VendorDiscovery />
        </div>

        {hasActiveCarts && (
          <div ref={cartPanelRef} className={`cart-panel-container ${showCartPanel ? 'expanded' : ''}`}>
            <CartPanel />
          </div>
        )}
      </div>
    </div>
  );
}

export default ShoppingPage;