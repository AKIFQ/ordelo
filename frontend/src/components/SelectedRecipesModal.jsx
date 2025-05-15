// src/components/SelectedRecipesModal.jsx

import { useState, useEffect } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { useRecipes } from "../context/RecipeContext";

function SelectedRecipesModal() {
  const [isOpen, setIsOpen] = useState(false);
  const { selectedRecipes, removeFromSelected, clearSelectedRecipes, addToShoppingList } = useRecipes();
  const [recipeServings, setRecipeServings] = useState({});
  const navigate = useNavigate();
  const location = useLocation();

  const showOnPages = ['/', '/saved-recipes', `/recipe/${location.pathname.split('/')[2]}`];
  const shouldShow = location.pathname.startsWith('/recipe/') || showOnPages.includes(location.pathname);

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (e.target.classList.contains('modal-overlay')) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  useEffect(() => {
    const initialServings = {};
    selectedRecipes.forEach(recipe => {
      const recipeId = recipe.id || recipe.recipe_id;
      initialServings[recipeId] = recipe.servings || recipe.serving_size || 1;
    });
    setRecipeServings(initialServings);
  }, [selectedRecipes]);

  const handleIncreaseServing = (recipeId) => {
    setRecipeServings(prev => ({
      ...prev,
      [recipeId]: prev[recipeId] + 1
    }));
  };

  const handleDecreaseServing = (recipeId) => {
    setRecipeServings(prev => ({
      ...prev,
      [recipeId]: Math.max(1, prev[recipeId] - 1)
    }));
  };

  const mergeIngredients = (ingredients) => {
    const merged = {};

    ingredients.forEach(ingredient => {
      const key = `${ingredient.name?.toLowerCase() || ''}-${ingredient.unit || ''}`;

      if (!merged[key]) {
        merged[key] = { ...ingredient };
      } else {
        merged[key].amount += ingredient.amount;
      }
    });

    return Object.values(merged);
  };

  const handleGenerateShoppingList = () => {
    const adjustedIngredients = selectedRecipes.flatMap(recipe => {
      const recipeId = recipe.id || recipe.recipe_id;
      const multiplier = recipeServings[recipeId] || 1;
      
      // Handle Spoonacular format
      if (recipe.extendedIngredients) {
        return recipe.extendedIngredients.map(ing => ({
          id: ing.id,
          name: ing.name,
          amount: (ing.measures?.metric?.amount || 1) * multiplier,
          unit: ing.measures?.metric?.unitShort || ing.unit || '',
          image: ing.image || ''
        }));
      }
      
      // Handle our custom format
      if (recipe.items) {
        return recipe.items.map(item => ({
          id: item.ingredient_id,
          name: item.name,
          amount: item.quantity * multiplier,
          unit: item.unit,
          unit_quantity: item.unit_quantity,
          price: item.price
        }));
      }
      
      return [];
    });

    const mergedIngredients = mergeIngredients(adjustedIngredients);
    addToShoppingList(mergedIngredients);
    setIsOpen(false);
    navigate('/shopping-list');
  };

  if (!shouldShow) return null;

  return (
    <>
      <button 
        className={`selected-recipes-fab ${isOpen ? 'open' : ''}`}
        onClick={() => setIsOpen(!isOpen)}
        aria-label="Selected recipes"
      >
        <i className="fas fa-shopping-basket"></i>
        <span className="selected-count">{selectedRecipes.length}</span>
      </button>

      {isOpen && selectedRecipes.length > 0 && (
        <div className="modal-overlay selected-recipes-overlay">
          <div className="selected-recipes-modal">
            <div className="modal-header">
              <h3 className="modal-header-text">Selected Recipes ({selectedRecipes.length})</h3>
              <button className="close-btn" onClick={() => setIsOpen(false)} aria-label="Close">
                <i className="fas fa-times"></i>
              </button>
            </div>

            <div className="selected-recipes-list">
              {selectedRecipes.map(recipe => {
                const recipeId = recipe.id || recipe.recipe_id;
                return (
                  <div key={recipeId} className="selected-recipe-item">
                    {recipe.extendedIngredients && (
                      <img 
                        src={recipe.image || '/assets/no-recipe-img.png'}
                        alt={recipe.title}
                        className="selected-recipe-img"
                        onError={(e) => {
                          e.target.onerror = null;
                          e.target.src = '/assets/no-recipe-img.png';
                        }}
                      />
                    )}
                    <div className="selected-recipe-details">
                      <h4>{recipe.title}</h4>
                      <span>{recipe.readyInMinutes || recipe.preparation_time || '30'} min</span>
                      <div className="servings-control">
                        <button onClick={() => handleDecreaseServing(recipeId)} className="serving-btn">-</button>
                        <span className="serving-count">{recipeServings[recipeId]}x</span>
                        <button onClick={() => handleIncreaseServing(recipeId)} className="serving-btn">+</button>
                      </div>
                    </div>
                    <button 
                      className="remove-selected-btn"
                      onClick={() => removeFromSelected(recipeId)}
                      aria-label={`Remove ${recipe.title}`}
                    >
                      <i className="fas fa-times"></i>
                    </button>
                  </div>
                );
              })}
            </div>

            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={clearSelectedRecipes}>
                Clear All
              </button>
              <button className="btn btn-primary" onClick={handleGenerateShoppingList}>
                <i className="fas fa-shopping-basket"></i> Generate Shopping List
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

export default SelectedRecipesModal;
