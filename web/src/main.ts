import { mount } from 'svelte'
import App from './App.svelte'
import './lib/prefs.svelte' // applies the saved theme before first paint
import './app.css'

mount(App, { target: document.getElementById('app')! })
