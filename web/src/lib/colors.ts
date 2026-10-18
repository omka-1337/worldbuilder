import { store } from './store.svelte'

// Colors offered for article types: mid-tone, so they read on both the
// light and the dark theme.
export const palette = [
  { color: '#d9534f', name: 'Червоний' },
  { color: '#e08a1e', name: 'Помаранчевий' },
  { color: '#c9a227', name: 'Золотий' },
  { color: '#3f9b5c', name: 'Зелений' },
  { color: '#2a9d8f', name: 'Бірюзовий' },
  { color: '#3b82c4', name: 'Синій' },
  { color: '#4a5bd4', name: 'Індиго' },
  { color: '#8e5bd4', name: 'Фіолетовий' },
  { color: '#c2559c', name: 'Рожевий' },
  { color: '#7a6f62', name: 'Сірий' },
]

// A type's color, or the accent color for types without one.
export const typeColor = (type?: string) => (type && store.world?.types[type]?.color) || 'var(--accent)'
