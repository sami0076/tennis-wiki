import '@testing-library/jest-dom/vitest'

// jsdom implements no layout, so it has no scrollIntoView. The components that
// call it only need it to exist.
Element.prototype.scrollIntoView = () => {}
