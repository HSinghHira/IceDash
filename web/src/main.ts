import { createApp } from 'vue'
import App from './App.vue'
// Fonts ship inside the image: the dashboard makes no calls to other sites.
import '@fontsource-variable/inter/wght.css'
import '@fontsource/unbounded/latin-800.css'
import './styles/tokens.css'
import './styles/base.css'

createApp(App).mount('#app')
