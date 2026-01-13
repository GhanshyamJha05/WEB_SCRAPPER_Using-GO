# Issues and Improvements Needed

## Bugs

1.  **Global State Management (Critical)**:
    - Variables like `currentResults`, `currentURL`, `currentSelector`, and `scrapeDuration` are defined globally.
    - **Impact**: If two users (or two tabs) use the scraper simultaneously, they will overwrite each other's data, leading to incorrect results being displayed.
    - **Fix**: Move these variables into the request handler scope and pass them to the template.

2.  **Theme Persistence**:
    - The Dark/Light mode toggle only updates the class on the client side temporarily or relies on a global `darkMode` variable on the server that isn't properly synced or scoped to the user.
    - **Fix**: Use `localStorage` to persist the theme preference on the client side, applying it immediately on page load.

3.  **Error Handling**:
    - Errors during scraping (e.g., invalid URL) allow the handler to write raw error strings (`fmt.Fprintf(w, "Error...")`) directly to the response, potentially breaking the HTML structure.
    - **Fix**: Pass error messages to the template and render them gracefully within the UI nicely (e.g., a simplified error alert).

4.  **Input Validation**:
    - The URL input does not fully validte input before attempting a request (though `http.Get` handles some, better feedback is needed).

## UI/UX Improvements

1.  **Separation of Concerns**:
    - HTML and CSS are embedded as string literals in `main.go`. This makes the code hard to read and maintain.
    - **Improvement**: Move HTML to `templates/index.html` and use Go's `html/template` package.

2.  **Modern Aesthetics**:
    - The current UI is functional but basic.
    - **Improvement**: Implement a modern, responsive design using CSS variables, flexbox/grid, and a refined color palette (including a better Dark Mode). Add hover effects and transitions.

3.  **User Experience**:
    - Add a loading state indicator when scraping.
    - Improve the result cards layout.
