<script setup lang="ts">
const route = useRoute()
</script>

<template>
  <div class="app-shell">
    <a
      class="skip-link"
      href="#main-content"
    >
      Skip to content
    </a>
    <header class="site-header">
      <div class="header-inner">
        <div class="topbar">
          <NuxtLink
            to="/"
            class="brand"
            aria-label="Crouton home"
          >
            <img
              class="brand-logo"
              src="/brand/crouton-logo.svg"
              alt="Crouton Digital"
              width="64"
              height="64"
            />
            <span class="brand-wordmark">
              Crouton Digital
              <span class="brand-sub">Stellar Explorer</span>
            </span>
          </NuxtLink>
          <div class="chain-context">
            <span class="chain-name">Stellar</span>
            <span class="network-badge"><span class="live-dot" /> Testnet</span>
          </div>
          <GlobalSearch />
        </div>
        <div class="header-navigation">
          <nav aria-label="Main navigation">
            <NuxtLink
              to="/"
              :class="{ active: route.path === '/' }"
              :aria-current="route.path === '/' ? 'page' : undefined"
            >
              Overview
            </NuxtLink>
            <NuxtLink
              to="/ledgers"
              :class="{ active: route.path.startsWith('/ledgers') }"
              :aria-current="route.path.startsWith('/ledgers') ? 'page' : undefined"
            >
              Ledgers
            </NuxtLink>
            <NuxtLink
              to="/transactions"
              :class="{
                active: route.path.startsWith('/transactions') || route.path.startsWith('/tx/'),
              }"
              :aria-current="
                route.path.startsWith('/transactions') || route.path.startsWith('/tx/')
                  ? 'page'
                  : undefined
              "
            >
              Transactions
            </NuxtLink>
            <NuxtLink
              to="/assets"
              :class="{ active: route.path.startsWith('/assets') }"
              :aria-current="route.path.startsWith('/assets') ? 'page' : undefined"
            >
              Assets
            </NuxtLink>
            <NuxtLink
              to="/nfts"
              :class="{ active: route.path.startsWith('/nfts') }"
              :aria-current="route.path.startsWith('/nfts') ? 'page' : undefined"
            >
              NFTs
            </NuxtLink>
            <NuxtLink
              to="/transfers"
              :class="{ active: route.path.startsWith('/transfers') }"
              :aria-current="route.path.startsWith('/transfers') ? 'page' : undefined"
            >
              Token movements
            </NuxtLink>
            <NuxtLink
              to="/events"
              :class="{ active: route.path.startsWith('/events') }"
              :aria-current="route.path.startsWith('/events') ? 'page' : undefined"
            >
              Events
            </NuxtLink>
          </nav>
          <a
            class="docs-link"
            href="https://developers.stellar.org/docs/data/apis/rpc"
            target="_blank"
            rel="noopener noreferrer"
          >
            Stellar RPC docs ↗
          </a>
        </div>
      </div>
    </header>
    <div class="main-shell">
      <main
        id="main-content"
        tabindex="-1"
      >
        <NuxtPage />
      </main>
      <footer>
        <NuxtLink
          class="footer-brand"
          to="/"
        >
          Crouton Digital <span class="muted">/ Stellar Explorer</span>
        </NuxtLink>
        <span>Testnet · Indexed history only · Exact integer amounts</span>
      </footer>
    </div>
  </div>
</template>
