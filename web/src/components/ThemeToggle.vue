<script setup lang="ts">
import { Moon, Sun } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { dark, setTheme } from '@/composables/useApp'
import { Button } from '@/components/ui/button'
const { t } = useI18n()
</script>
<template>
  <Button
    variant="ghost"
    size="icon"
    :aria-label="dark ? t('common.light') : t('common.dark')"
    :title="dark ? t('common.light') : t('common.dark')"
    @click="setTheme(!dark)"
  >
    <Sun v-if="dark" />
    <Moon v-else />
  </Button>
</template>
