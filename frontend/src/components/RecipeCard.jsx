import { useState } from "react";
import { useRecipes } from "../context/RecipeContext";
import { useNavigate } from "react-router-dom";  

function RecipeCard({ recipe }) {
  const [isHovered, setIsHovered] = useState(false);
  const { addToSelectedRecipes, selectedRecipes } = useRecipes();
  const navigate = useNavigate(); 
  
  const isSelected = selectedRecipes.some(r => r.id === recipe.id);
  

  const openExternalRecipe = () => {
    navigate(`/recipe/${recipe.id}`);
  };
  
  return (
    <div 
      className="recipe-card" 
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <div className="recipe-image-container" style={{ position: 'relative' }}>
        <img 
          src={recipe.image || '/assets/placeholder-food.jpg'} 
          alt={recipe.title} 
          className="recipe-image" 
          onError={e => {
            e.target.onerror = null;
            e.target.src = '/assets/no-recipe-img.png';
          }}
        />
        {isHovered && (
          <div className="recipe-card-overlay">
            <button 
              onClick={openExternalRecipe} 
              className="btn btn-primary view-recipe-btn"
            >
              <i className="fas fa-external-link-alt"></i> View Recipe
            </button>
          </div>
        )}
      </div>
      
      <div className="recipe-content">
        <h3 className="recipe-title" onClick={openExternalRecipe}>
          {recipe.title}
        </h3>
        
        <div className="recipe-meta">
          <span>
            <i className="far fa-clock"></i> {recipe.readyInMinutes || '30'} min
          </span>
          <span>
            <i className="fas fa-fire"></i> {recipe.calories || '400'} cal
          </span>
        </div>
        
        <div className="recipe-buttons">
          <button 
            className={`btn ${isSelected ? 'btn-accent' : 'btn-secondary'}`}
            onClick={() => addToSelectedRecipes(recipe)}
            aria-label={isSelected ? "Selected" : "Select recipe"}
          >
            <i className="fas fa-shopping-basket"></i>
            {isSelected ? ' Selected' : ' Select'}
          </button>
        </div>
      </div>
    </div>
  );
}

export default RecipeCard;