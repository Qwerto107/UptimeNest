<script setup lang="ts">
defineProps<{ layout: 'status' | 'monitors' | 'panels' }>()
</script>

<template>
  <div class="page-skeleton" :data-layout="layout" aria-hidden="true">
    <div v-if="layout === 'status'" class="skeleton-status">
      <div class="pulse-summary">
        <span class="skeleton-line skeleton-dot" />
        <div class="skeleton-banner-copy"><span class="skeleton-line skeleton-banner-title" /><span class="skeleton-line skeleton-date" /></div>
        <span class="skeleton-line skeleton-refresh" />
      </div>
      <div class="pulse-group">
        <div class="pulse-group-heading"><span class="skeleton-line skeleton-group-name" /></div>
        <div class="pulse-service-list">
          <div class="pulse-list-surface" />
          <div v-for="row in 3" :key="row" class="pulse-service">
            <div class="pulse-service-heading">
              <div class="pulse-service-name-line"><span class="skeleton-line skeleton-name" /><span class="skeleton-line skeleton-address" /><span class="skeleton-line skeleton-pill" /></div>
              <div class="pulse-service-result"><span class="skeleton-line skeleton-uptime" /><span class="skeleton-line skeleton-state" /></div>
            </div>
            <div class="pulse-connection-meta skeleton-meta"><span v-for="pill in 4" :key="pill" class="skeleton-line skeleton-pill" /></div>
            <div class="skeleton-ticks" />
          </div>
        </div>
      </div>
    </div>
    <template v-else-if="layout === 'monitors'">
      <div class="monitor-overview">
        <div v-for="stat in 4" :key="stat" class="overview-stat"><span class="skeleton-line skeleton-caption" /><div><span class="skeleton-line skeleton-number" /><span class="skeleton-line skeleton-unit" /></div></div>
      </div>
      <div class="list-toolbar">
        <div class="search-input skeleton-control"><span class="skeleton-line skeleton-search-copy" /></div>
        <div v-for="filter in 2" :key="filter" class="filter-select skeleton-control"><span class="skeleton-line skeleton-filter-copy" /></div>
        <span class="skeleton-control skeleton-button" />
      </div>
      <div class="monitor-tree monitor-list">
        <div class="monitor-group-row"><span class="skeleton-line skeleton-group-name" /></div>
        <div v-for="row in 3" :key="row" class="monitor-list-row">
          <div class="monitor-list-info"><span class="skeleton-line skeleton-name" /><span class="skeleton-line skeleton-target" /></div>
          <span class="monitor-protocol skeleton-line skeleton-pill" />
          <div class="monitor-list-state"><span class="skeleton-line skeleton-state" /><span class="skeleton-line skeleton-small" /></div>
          <div class="monitor-list-history"><div class="skeleton-ticks" /><span class="skeleton-line skeleton-percent" /></div>
          <div class="row-actions"><span v-for="action in 5" :key="action" class="skeleton-action" /></div>
        </div>
      </div>
    </template>
    <div v-else class="entity-list">
      <div v-for="row in 3" :key="row" class="entity-row">
        <span class="skeleton-line skeleton-entity-icon" />
        <div class="entity-info"><span class="skeleton-line skeleton-name" /><span class="skeleton-line skeleton-target" /></div>
        <div class="row-actions"><span v-for="action in 3" :key="action" class="skeleton-action" /></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.skeleton-line, .skeleton-action { display: block; max-width: 100%; height: 12px; border-radius: 4px; background: var(--pulse-pill, var(--muted)); }
.skeleton-status { display: grid; gap: 28px; }
.skeleton-dot { width: 10px; height: 10px; flex: none; border-radius: 50%; }
.skeleton-banner-copy { display: grid; align-content: center; flex: 1; min-width: 0; min-height: 42px; gap: 8px; }
.skeleton-banner-title { width: 180px; height: 18px; }
.skeleton-date { width: 200px; height: 10px; }
.skeleton-refresh { width: 36px; height: 14px; flex: none; }
.skeleton-group-name { width: 80px; }
.skeleton-name { width: 100px; height: 14px; }
.skeleton-address { width: 120px; height: 10px; }
.skeleton-pill { width: 44px; height: 18px; border-radius: 999px; }
.skeleton-uptime { width: 60px; height: 11px; }
.skeleton-state { width: 54px; height: 20px; border-radius: 999px; }
.skeleton-meta > :nth-child(2) { width: 112px; }
.skeleton-ticks { height: 14px; width: 100%; background: repeating-linear-gradient(to right, var(--pulse-pill, var(--muted)) 0 3px, transparent 3px 6px); }
.skeleton-caption { width: 64px; height: 10px; margin: 2px 0 10px; }
.skeleton-number { width: 42px; height: 27px; }
.skeleton-unit { width: 26px; height: 10px; }
.overview-stat::before { background: var(--muted); }
.skeleton-control { display: flex; align-items: center; min-height: 36px; padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px; background: var(--card); }
.skeleton-search-copy { width: 180px; height: 10px; }
.skeleton-filter-copy { width: 72px; height: 10px; }
.skeleton-button { width: 36px; flex: none; }
.skeleton-target { width: 160px; height: 10px; margin-top: 7px; }
.skeleton-small { width: 40px; height: 10px; margin-top: 4px; }
.skeleton-percent { width: 54px; height: 10px; margin-left: auto; }
.skeleton-action { flex: none; width: 28px; height: 24px; margin: 2px; }
.skeleton-entity-icon { flex: none; width: 40px; height: 40px; border-radius: 10px; }
@media (max-width: 640px) { .skeleton-status { gap: 24px; } }
</style>
