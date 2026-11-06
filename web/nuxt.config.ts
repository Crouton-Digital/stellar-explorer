export default defineNuxtConfig({
  compatibilityDate: '2025-11-01',
  devtools: { enabled: false },
  css: ['~/assets/main.css'],
  runtimeConfig: {
    apiBase: 'http://localhost:8080',
    public: { apiBase: '/api/v1' },
  },
  app: {
    head: {
      title: 'Crouton · Stellar Explorer',
      htmlAttrs: { lang: 'en' },
      link: [{ rel: 'icon', type: 'image/svg+xml', href: '/brand/crouton-logo.svg' }],
      meta: [
        { name: 'theme-color', content: '#040806' },
        {
          name: 'description',
          content: 'Explore indexed Stellar Testnet ledgers, transactions and Soroban activity.',
        },
      ],
    },
  },
})
