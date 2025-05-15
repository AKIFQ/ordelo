import { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../../context/AuthContext";
import { firebaseConfig } from "../../firebase/firebase.js";
import { getCurrentLocation } from "../../components/getLocation.js";

const GOOGLE_MAPS_API_KEY = firebaseConfig.apiKey;

function VendorStore() {
  const [stores, setStores] = useState([]);
  const [activeTab, setActiveTab] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [storeName, setStoreName] = useState("");
  const [storeType, setStoreType] = useState("Delivery");
  const [latitude, setLatitude] = useState("");
  const [longitude, setLongitude] = useState("");
  const [address, setAddress] = useState("");
  
  const addressInputRef = useRef(null);
  const autocompleteRef = useRef(null);

  const [editingIngredientId, setEditingIngredientId] = useState(null);
  const [editFormData, setEditFormData] = useState({
    quantity: "",
    price: ""
  });

  const [loading, setLoading] = useState(true);
  const [formLoading, setFormLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const { user } = useAuth();
  const navigate = useNavigate();
  const [mapsApiLoaded, setMapsApiLoaded] = useState(false);

  const loadGoogleMapsScript = () => {
    if (window.google && window.google.maps && window.google.maps.places && window.google.maps.Geocoder) {
      setMapsApiLoaded(true);
      return;
    }
    if (document.getElementById("google-maps-script-vendorstore")) {
        if (window.google && window.google.maps) setMapsApiLoaded(true);
        return;
    }

    const script = document.createElement("script");
    script.id = "google-maps-script-vendorstore";
    script.src = `https://maps.googleapis.com/maps/api/js?key=${GOOGLE_MAPS_API_KEY}&libraries=places,geocoding&callback=initMapVendorStore`;
    script.async = true;
    script.defer = true;
    

    if (!window.initMapVendorStore) {
        window.initMapVendorStore = () => {
            setMapsApiLoaded(true);
            console.log("Google Maps API loaded successfully for VendorStore.");
        };
    }

    document.head.appendChild(script);
    
    script.onerror = () => {
        setError("Failed to load Google Maps. Address features may not work.");
        console.error("Google Maps script loading failed for VendorStore.");
    };
  };

  useEffect(() => {
    loadGoogleMapsScript();
  }, []);

  useEffect(() => {
    if (user) {
      fetchStores();
    }
  }, [user]);

  useEffect(() => {
    if (mapsApiLoaded && showForm && addressInputRef.current && !autocompleteRef.current) {
      try {
        const autocomplete = new window.google.maps.places.Autocomplete(
          addressInputRef.current,
          { types: ["geocode"] }
        );
        autocomplete.setFields(["address_components", "formatted_address", "geometry", "name"]);
        autocomplete.addListener("place_changed", handlePlaceSelect);
        autocompleteRef.current = autocomplete;
      } catch (e) {
        console.error("Error initializing Google Places Autocomplete:", e);
        setError("Could not initialize address autocomplete. Please try refreshing.");
      }
    }

    return () => {
      if (autocompleteRef.current && window.google && window.google.maps && window.google.maps.event) {
        window.google.maps.event.clearInstanceListeners(autocompleteRef.current);
      }
    };
  }, [mapsApiLoaded, showForm]); 


  const handlePlaceSelect = () => {
    if (!autocompleteRef.current) return;
    const place = autocompleteRef.current.getPlace();
    if (place && place.geometry && place.geometry.location) {
      setAddress(place.formatted_address || place.name || "");
      setLatitude(place.geometry.location.lat().toString());
      setLongitude(place.geometry.location.lng().toString());
      setError(null);
    } else {
      console.warn("Place not found or geometry missing after autocomplete selection.");
    }
  };

  const handleUseCurrentLocation = async () => {
    if (!mapsApiLoaded) {
        setError("Maps API not loaded yet. Please wait or try refreshing.");
        return;
    }
    setFormLoading(true);
    setError(null);
    try {
      const { lat, lng } = await getCurrentLocation();
      setLatitude(lat.toString());
      setLongitude(lng.toString());


      const geocoder = new window.google.maps.Geocoder();
      geocoder.geocode({ location: { lat, lng } }, (results, status) => {
        if (status === "OK") {
          if (results && results[0]) {
            setAddress(results[0].formatted_address);
            if (addressInputRef.current) {
                addressInputRef.current.value = results[0].formatted_address;
            }
          } else {
            setError("No address found for your current location.");
            setAddress("");
            if (addressInputRef.current) addressInputRef.current.value = "";
          }
        } else {
          setError(`Geocoder failed due to: ${status}. Try entering address manually.`);
          setAddress("");
          if (addressInputRef.current) addressInputRef.current.value = "";
        }
        setFormLoading(false);
      });
    } catch (geolocationError) {
      let message = "Could not get current location. ";


      if (typeof geolocationError === 'string') {
          message = geolocationError;
      } else if (geolocationError.message) {
          message = geolocationError.message;
      } else {
          message = "An unknown error occurred while fetching location.";
      }
      setError(message + " Please enter address manually or check browser settings.");
      setFormLoading(false);
    }
  };

  const fetchStores = async () => {
    try {
      setLoading(true);
      const response = await fetch("http://localhost:8080/vendor/stores", {
        method: "GET",
        headers: { Authorization: `Bearer ${user.token}` },
      });
      if (!response.ok) throw new Error("Failed to fetch stores");
      const data = await response.json();
      const normalizedStores = (data.value || []).map((store) => ({
        ...store, id: store.store_id || store._id?.$oid || store._id,
      }));
      setStores(normalizedStores);
      if (normalizedStores.length) setActiveTab(normalizedStores[0]);
      setError(null);
    } catch (err) {
      setError("Failed to load stores. Please try again.");
      console.error("Error fetching stores:", err);
    } finally {
      setLoading(false);
    }
  };

  const resetCreateStoreForm = () => {
    setStoreName("");
    setStoreType("Delivery");
    setLatitude("");
    setLongitude("");
    setAddress("");
    if (addressInputRef.current) {
      addressInputRef.current.value = "";
    }
  };

  const handleToggleForm = () => {
    const newShowFormState = !showForm;
    setShowForm(newShowFormState);
    if (!newShowFormState) {
        resetCreateStoreForm();
    } else {
        setError(null);
        handleUseCurrentLocation(); 
    }
  };

  const handleCreateStore = async () => {
    setError(null);
    if (!storeName.trim()) {
      setError("Store name is required.");
      return;
    }
    if (!latitude || !longitude) {
      setError("Location (Latitude and Longitude) is required. Use current location or enter an address to auto-fill.");
      return;
    }
     if (!address.trim()) {
      setError("Address is required.");
      return;
    }

    const storePayload = {
      stores: [{
        name: storeName,
        store_type: storeType,
        address: address,
        location: { type: "Point", coordinates: [parseFloat(longitude), parseFloat(latitude)] },
        items: [],
      }],
    };

    try {
      setFormLoading(true);
      const response = await fetch("http://localhost:8080/vendor/stores", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${user.token}` },
        body: JSON.stringify(storePayload),
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ message: "Failed to create store due to server error." }));
        throw new Error(errorData.message || "Failed to create store.");
      }
      await response.json();
      setShowForm(false);
      resetCreateStoreForm();
      setSuccess("Store created successfully!");
      setTimeout(() => setSuccess(null), 3000);
      fetchStores();
    } catch (err) {
      setError(err.message || "Failed to create store. Please try again.");
      console.error("Error creating store:", err);
    } finally {
      setFormLoading(false);
    }
  };


  const handleEditClick = (item) => {
    setEditingIngredientId(item.ingredient_id);
    setEditFormData({ unit_quantity: item.unit_quantity, price: item.price });
  };

  const handleEditChange = (e) => {
    const { name, value } = e.target;
    setEditFormData(prev => ({ ...prev, [name]: value }));
  };

  const handleSaveEdit = async () => {
    const itemToUpdate = activeTab?.items?.find(item => item.ingredient_id === editingIngredientId);
    if (!itemToUpdate || !activeTab) return;

    const payload = {
      stores: [
        {
          store_id: activeTab.store_id,
          items: [
            {
              ingredient_id: itemToUpdate.ingredient_id,
              name: itemToUpdate.name,
              unit: itemToUpdate.unit,
              unit_quantity: itemToUpdate.unit_quantity || 0,
              price: parseFloat(editFormData.price),
              quantity: parseFloat(editFormData.quantity)
            }
          ]
        }
      ]
    };
    console.log("Saving edited ingredient:", payload);
    try {
        setFormLoading(true);
        const response = await fetch("http://localhost:8080/vendor/stores", {
          method: "PUT",
          headers: { "Content-Type": "application/json", Authorization: `Bearer ${user.token}` },
          body: JSON.stringify(payload),
        });
        if (!response.ok) throw new Error("Failed to update item.");
        setSuccess("Item updated!");
        setTimeout(() => setSuccess(null), 3000);
        fetchStores();
        setEditingIngredientId(null);
    } catch (e) {
        setError("Failed to update item.");
    } finally {
        setFormLoading(false);
    }
  };

  const handleCancelEdit = () => {
    setEditingIngredientId(null);
    setEditFormData({
      quantity: "",
      price: ""
    });
  };

  const formatCurrency = (amount) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency', currency: 'USD', minimumFractionDigits: 2
    }).format(amount || 0);
  };


  if (loading && stores.length === 0) {
    return (
      <div className="loading-spinner">
        <div className="spinner"></div>
        <p>Loading store data...</p>
      </div>
    );
  }

  return (
    <div className="vendor-container">
      <div className="action-bar">
        <h2 className="page-title">Manage Stores</h2>
        <button onClick={handleToggleForm} className="btn btn-primary toggle-form-btn">
          {showForm ? <><i className="fas fa-times"></i><span>Cancel</span></>
                     : <><i className="fas fa-plus"></i><span>Create Store</span></>}
        </button>
      </div>

      {error && (
        <div className="error-message">
          <i className="fas fa-exclamation-circle"></i>
          <span>{error}</span>
          <button onClick={() => setError(null)} className="close-alert-btn">×</button>
        </div>
      )}
      {success && (
        <div className="success-message">
          <i className="fas fa-check-circle"></i>
          <span>{success}</span>
          <button onClick={() => setSuccess(null)} className="close-alert-btn">×</button>
        </div>
      )}

      {showForm && (
        <div className="vendor-section create-store-form-section">
          <div className="vendor-panel">
            <div className="vendor-panel-header"><h3>Create New Store</h3></div>
            <div className="vendor-panel-body">
              <div className="form-grid">
                <div className="form-group">
                  <label htmlFor="storeName">Store Name</label>
                  <input id="storeName" type="text" placeholder="Enter Store Name" value={storeName}
                         onChange={(e) => setStoreName(e.target.value)} className="form-control" required/>
                </div>
                <div className="form-group">
                  <label htmlFor="storeType">Store Type</label>
                  <select id="storeType" value={storeType} onChange={(e) => setStoreType(e.target.value)}
                          className="form-control">
                    <option value="Delivery">Delivery</option>
                    <option value="Pickup">Pickup</option>
                  </select>
                </div>
                
                <div className="form-group full-width">
                  <label htmlFor="address">Store Address</label>
                  <input id="address" type="text" ref={addressInputRef}
                         placeholder="Start typing address or use current location"
                         defaultValue={address}
                         onChange={(e) => setAddress(e.target.value)}
                         className="form-control" disabled={!mapsApiLoaded}/>
                  {!mapsApiLoaded && <small className="form-text text-muted">Loading address service...</small>}
                </div>

                <div className="form-group">
                   <button type="button" onClick={handleUseCurrentLocation} className="btn btn-secondary btn-sm"
                           disabled={formLoading || !mapsApiLoaded}>
                    {formLoading && mapsApiLoaded ? (<><span className="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span><span> Fetching...</span></>)
                                   : (<><i className="fas fa-map-marker-alt"></i><span> Use My Current Location</span></>)}
                  </button>
                </div>
                 <div className="form-group"></div> {/* Spacer */}

                <div className="form-group">
                  <label htmlFor="latitude">Latitude</label>
                  <input id="latitude" type="text" placeholder="Latitude (auto-filled)" value={latitude}
                         onChange={(e) => setLatitude(e.target.value)} className="form-control" readOnly/>
                </div>
                <div className="form-group">
                  <label htmlFor="longitude">Longitude</label>
                  <input id="longitude" type="text" placeholder="Longitude (auto-filled)" value={longitude}
                         onChange={(e) => setLongitude(e.target.value)} className="form-control" readOnly/>
                </div>
              </div>
            </div>
            <div className="panel-footer">
              <button onClick={handleCreateStore} className="btn btn-primary"
                      disabled={formLoading || !storeName.trim() || !latitude || !longitude || !address.trim()}>
                {formLoading ? <div className="spinner-sm"></div> : <><i className="fas fa-save"></i><span>Save Store</span></>}
              </button>
            </div>
          </div>
        </div>
      )}

      {!showForm && stores.length > 0 && (
        <div className="tabs">
          {stores.map((store) => (
            <button key={store.id}
                    className={`tab-button ${activeTab?.id === store.id ? "active" : ""}`}
                    onClick={() => setActiveTab(store)}>
              {store.name} ({store.items?.length || 0} items)
            </button>
          ))}
        </div>
      )}

      {!showForm && stores.length === 0 && !loading && (
        <div className="empty-state">
          <i className="fas fa-store"></i><h3>No stores available</h3>
          <p>Create your first store to start managing inventory.</p>
        </div>
      )}

      {!showForm && activeTab && (
        <>
          <div className="store-details">
            <div className="vendor-panel">
              <div className="vendor-panel-header"><h3>Store Details</h3></div>
              <div className="vendor-panel-body"><div className="data-grid">
                <div className="data-label">Name:</div><div className="data-value">{activeTab.name}</div>
                <div className="data-label">Type:</div><div className="data-value">{activeTab.store_type}</div>
                <div className="data-label">Items:</div><div className="data-value">{activeTab.items?.length || 0}</div>
                <div className="data-label">Location:</div>
                <div className="data-value">
                  {activeTab.location?.coordinates ? `(${activeTab.location.coordinates[1]}, ${activeTab.location.coordinates[0]})` : 'Not specified'}
                </div>
                 <div className="data-label">Address:</div><div className="data-value">{activeTab.address || 'Not specified'}</div>
              </div></div>
            </div>
          </div>

          <div className="vendor-section">
            <h3 className="vendor-section-title">Inventory for {activeTab.name}</h3>
            {activeTab.items && activeTab.items.length > 0 ? (
              <div className="ingredients-table-container">
                <table className="ingredients-table">
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Unit Quantity</th>
                     <th>Quantity</th>
                      <th>Price</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {activeTab.items.map((item, idx) => (
                      <tr key={item.ingredient_id || idx}>
                        <td>{item.name}</td>
                        {editingIngredientId === item.ingredient_id ? (
                          <>
                            <td>
                     <span className="unit-label">{item.unit_quantity}</span>
                      <span className="unit-label">{item.unit}</span>
                    </td>
                    <td>
                      <input
                        type="number"
                        name="quantity"
                        value={editFormData.quantity}
                        onChange={handleEditChange}
                      />
                    </td>
                    <td>
                      <input
                        type="number"
                        name="price"
                        value={editFormData.price}
                        onChange={handleEditChange}
                      />
                    </td>
                    <td>
                      <div className="table-actions">
                        <button onClick={handleSaveEdit} className="btn btn-sm btn-primary">
                          <i className="fas fa-save"></i>
                          <span>Save</span>
                        </button>
                        <button onClick={handleCancelEdit} className="btn btn-sm btn-secondary">
                          <i className="fas fa-times"></i>
                          <span>Cancel</span>
                        </button>
                      </div>
                    </td>
                  </>
                ) : (
                  <>
                    <td>{item.unit_quantity} {item.unit}</td>
                    <td>{item.quantity}</td>
                    <td>
                      <span className="price-inline">
                        <span className="dollar-sign">$</span>
                        {formatCurrency(item.price).replace('$', '')}
                      </span>
                    </td>
                            <td>
                              <button onClick={() => handleEditClick(item)} className="btn btn-sm btn-primary"><i className="fas fa-edit"></i>
                              <span>Edit</span>
                              </button>
                            </td>
                          </>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="empty-state">
                <i className="fas fa-box-open"></i><h3>No ingredients assigned yet</h3>
                <p>Add products to your inventory to get started.</p>
                <button onClick={() => navigate("/vendor/add-inventory")} className="btn btn-primary mt-md">
                  <i className="fas fa-plus"></i><span>Add Products</span>
                </button>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

export default VendorStore;