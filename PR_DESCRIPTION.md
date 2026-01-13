# UI Overhaul and Bug Fixes

## Summary
This PR revamps the entire UI with a modern, responsive design and fixes critical concurrency bugs caused by global state management.

## Changes

### 🎨 UI/UX Improvements
- **New Modern Design**: Replaced the basic embedded HTML with a standalone `templates/index.html` file using a modern dark-themed design (Inter font, glassmorphism, responsive grid).
- **Client-Side Theme Persistence**: Moved theme toggling to client-side `localStorage`, ensuring the user's preference is remembered without server state issues.
- **Improved Results Display**: Results are now shown in a clean grid layout.
- **Loading State**: Added a visual loading indicator during scraping.

### 🐛 Bug Fixes
- **Concurrency Fix**: Removed global variables (`currentResults`, `currentURL`, etc.) that caused race conditions. Data is now scoped to the request.
- **Error Handling**: proper error messages are now passed to the template instead of writing raw strings to the response, which broke the HTML structure previously.
- **URL Normalization**: Improved handling of relative links in scraped results.
- **Code Structure**: Refactored `main.go` to use `html/template` best practices.

## Testing
- Verified `go build` succeeds.
- Manual testing recommended:
    1. Run `go run main.go`.
    2. Open `http://localhost:8080`.
    3. Try scraping sites in multiple tabs simultaneously to verify isolation.
    4. Toggle themes and refresh to verify persistence.
