<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
// @ts-expect-error vue-tel-input does not have TypeScript declarations
import { VueTelInput } from 'vue-tel-input';
import 'vue-tel-input/vue-tel-input.css';

const _props = withDefaults(defineProps<Props>(), {
  placeholder: 'Enter phone number',
  disabled: false,
  defaultCountry: 'US',
  mode: 'international',
  onlyCountries: () => [],
  readonly: false,
  fieldName: 'phone',
  dropdownOffset: 0,
  dataCy: 'tel-input',
})

const emits = defineEmits<Emits>()

// const { getUser } = useMainStore()

const defaultCountry = 'US'

const dropdownOffsetPx = computed(() => `${_props.dropdownOffset ?? 0}px`)

// Type for country change event
interface CountryData {
  name: string
  iso2: string
  dialCode: string
  priority: number
  areaCodes: string[] | null
}

interface PhoneObject {
  valid: boolean
  country: CountryData
  countryCode: string
  nationalNumber: string
  formatInternational: string
  formatNational: string
  uri: string
  e164: string
}

interface Props {
  placeholder?: string
  disabled?: boolean
  defaultCountry?: string
  onlyCountries?: string[]
  readonly?: boolean
  mode?: 'international' | 'national'
  form?: any // Form instance from vee-validate
  fieldName?: string // Field name for validation
  dropdownOffset?: number // Offset for dropdown position in pixels
  dataCy?: string // Data-cy attribute for Cypress testing
  // Allow alphabetic characters for contact search scenarios
  allowAlphabets?: boolean
}

interface Emits {
  validate: [value: boolean]
}

// Track if search input is focused
const isSearchInputFocused = ref(false)

// Use defineModel with computed getter/setter to optionally allow alphabetic characters
const modelValue = defineModel<string>({
  default: '',
  get(value: string) {
    // Return the value as-is for display
    return value || ''
  },
  set(value: string) {
    // Don't update model value if search input is focused
    if (isSearchInputFocused.value) {
      return value
    }
    // If alphabets are allowed, return raw value (used for contact search)
    if (_props.allowAlphabets) {
      return value
    }
    // Otherwise filter out alphabetic characters, keep only numbers and common symbols
    const filteredValue = value.replace(/[a-z]/gi, '')
    return filteredValue
  },
})

// Handle phone validation
function handlePhoneValidation(phoneObj: PhoneObject) {
  emits('validate', phoneObj.valid)
  // If form prop is provided, use setErrors for validation
  if (_props.form && _props.fieldName) {
    if (!phoneObj.valid) {
      _props.form.setErrors({
        last_name: 'Invalid phone number',
        [_props.fieldName]: 'Invalid phone number',
      })
    }
    else if (phoneObj.valid) {
      // Clear the error if phone is valid
      _props.form.setErrors({
        [_props.fieldName]: undefined,
      })
    }
  }

  return phoneObj.valid
}

// Function to handle focus events on search input
function handleSearchInputFocus() {
  isSearchInputFocused.value = true
}

function handleSearchInputBlur() {
  isSearchInputFocused.value = false
}

// Set up event listeners for search input focus/blur
onMounted(() => {
  // Use a slight delay to ensure the DOM is fully rendered
  setTimeout(() => {
    const searchInput = document.querySelector('.vti__search_box')
    if (searchInput) {
      searchInput.addEventListener('focus', handleSearchInputFocus)
      searchInput.addEventListener('blur', handleSearchInputBlur)
    }
  }, 100)
})

onUnmounted(() => {
  const searchInput = document.querySelector('.vti__search_box')
  if (searchInput) {
    searchInput.removeEventListener('focus', handleSearchInputFocus)
    searchInput.removeEventListener('blur', handleSearchInputBlur)
  }
})
</script>

<template>
  <VueTelInput
    v-model="modelValue"
    :mode="mode"
    auto-default-country
    :only-countries="onlyCountries"
    :input-options="{
      'id': 'phone-input',
      'name': 'phone',
      'readonly': readonly,
      'placeholder': placeholder,
      'data-cy': `${dataCy}-input`,
    }"
    :default-country="defaultCountry"
    :enabled-country-code="true"
    :valid-characters-only="!_props.allowAlphabets"
    :dropdown-options="{
      'searchPlaceholder': 'Search countries',
      'name': 'country',
      'showDialCodeInList': true,
      'showFlags': true,
      'showSearchBox': true,
      'readonly': readonly,
      'disabled': readonly,
      'data-cy': `${dataCy}-country-dropdown`,
    }"
    :readonly="readonly"
    :disabled="disabled"
    :placeholder="placeholder"
    class="tel-input"
    :data-cy="dataCy"
    @validate="handlePhoneValidation"
  />
</template>

<style scoped>
@reference "tailwindcss";
/* Override vue-tel-input styles to match shadcn Input and Select components */
:deep(.vue-tel-input) {
  @apply flex w-full rounded-xs border ring-offset-2;
  @apply focus-within:ring-2 focus-within:ring-offset-2;
  border-color: hsl(var(--input));
  background-color: hsl(var(--background));
}

/* Default sizing: compact */
:deep(.vue-tel-input.tel-input) {
  @apply text-xs;
  min-height: 2.25rem; /* ~h-9 */
  line-height: 1.1;
}

:deep(.vue-tel-input:focus-within) {
  /* Set ring color via CSS var so Tailwind ring-2 uses it */
  --tw-ring-color: hsl(var(--ring));
}

:deep(.vue-tel-input.disabled) {
  @apply cursor-not-allowed opacity-50;
}

:deep(.vue-tel-input .vti__dropdown) {
  @apply flex items-center justify-center px-2 border-r transition-colors;
  border-radius: 0.375rem 0 0 0.375rem;
  min-width: 52px;
  border-color: hsl(var(--input));
  background-color: hsl(var(--background));
}

:deep(.vue-tel-input .vti__dropdown:hover) {
  background-color: hsl(var(--accent));
}

:deep(.vue-tel-input .vti__dropdown .vti__selection) {
  @apply flex items-center gap-1 text-xs;
  color: hsl(var(--foreground));
  font-size: 14px !important;
}

:deep(.vue-tel-input .vti__dropdown .vti__country-code) {
  @apply text-xs;
  color: hsl(var(--muted-foreground));
  font-size: 14px !important;
}

:deep(.vue-tel-input .vti__dropdown .vti__flag) {
  margin-right: 4px;
}

:deep(.vue-tel-input .vti__input) {
  @apply flex-1 bg-transparent px-2 py-1.5 text-xs focus:outline-none;
  color: hsl(var(--foreground));
  font-size: 14px !important;
  /* file:* utilities replaced with explicit CSS */
  &::file-selector-button {
    border: 0;
    background: transparent;
    color: hsl(var(--foreground));
    font-size: 0.875rem;
    font-weight: 500;
  }
  border: none !important;
  box-shadow: none !important;
  outline: none !important;
}

:deep(.vue-tel-input .vti__input::placeholder) {
  color: hsl(var(--muted-foreground));
}

:deep(.vue-tel-input .vti__input:disabled) {
  @apply cursor-not-allowed opacity-50;
}

/* Dynamic dropdown positioning based on prop */
:deep(.vue-tel-input .vti__dropdown-list.below) {
  left: v-bind(dropdownOffsetPx) !important;
}

:deep(.vue-tel-input .vti__dropdown-list) {
  @apply border rounded-xs shadow-lg mt-1;
  z-index: 50;
  max-height: 180px;
  overflow-y: auto;
  border-color: hsl(var(--input));
  background-color: hsl(var(--background));
  font-size: 14px !important;
}

/* Country dropdown items - matching ComboboxItem */
:deep(.vue-tel-input .vti__dropdown-item) {
  @apply relative flex cursor-default select-none items-center rounded-sm text-xs outline-none;
  @apply gap-2 transition-colors;
  padding: 2px 8px !important;
  min-height: 24px !important;
  line-height: 1.1 !important;
  font-size: 14px !important;
}

:deep(.vue-tel-input .vti__dropdown-item:hover) {
  background-color: hsl(var(--accent));
  color: hsl(var(--accent-foreground));
}

:deep(.vue-tel-input .vti__dropdown-item.highlighted) {
  background-color: hsl(var(--accent));
  color: hsl(var(--accent-foreground));
}

:deep(.vue-tel-input .vti__dropdown-item:focus) {
  background-color: hsl(var(--accent));
  color: hsl(var(--accent-foreground));
}

:deep(.vue-tel-input .vti__dropdown-item .vti__flag) {
  @apply shrink-0 w-4 h-3;
  margin-right: 6px;
}

/* Country name and code styling */
:deep(.vue-tel-input .vti__dropdown-item strong) {
  @apply font-medium flex-1;
  font-size: inherit !important;
  font-weight: 500 !important;
  color: hsl(var(--foreground));
}

:deep(.vue-tel-input .vti__dropdown-item span:last-child) {
  @apply ml-auto;
  font-size: 10px !important;
  font-weight: 400;
  color: hsl(var(--muted-foreground));
}

/* Highlighted state text colors */
:deep(.vue-tel-input .vti__dropdown-item.highlighted strong) {
  color: hsl(var(--accent-foreground));
}

:deep(.vue-tel-input .vti__dropdown-item.highlighted span:last-child) {
  color: color-mix(in srgb, hsl(var(--accent-foreground)) 70%, transparent);
}

/* Search box container */
:deep(.vue-tel-input .vti__search_box_container) {
  @apply relative w-full;
}

/* Search box in dropdown - matching ComboboxInput */
:deep(.vue-tel-input .vti__search_box) {
  @apply w-full pl-8 pr-2 py-1.5 text-xs border-0 border-b bg-transparent;
  border-color: hsl(var(--border));
  @apply focus:outline-none focus:ring-0;
  font-size: 14px !important;
  &:focus {
    border-color: hsl(var(--ring));
  }
  /* placeholder color set below via ::placeholder */
  border-radius: 0;
  height: 28px;
}

/* Search icon */
:deep(.vue-tel-input .vti__search_box_container::before) {
  content: '';
  @apply absolute left-2 top-1/2 transform -translate-y-1/2;
  @apply w-3.5 h-3.5;
  color: hsl(var(--muted-foreground));
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke-width='1.5' stroke='currentColor'%3e%3cpath stroke-linecap='round' stroke-linejoin='round' d='m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z' /%3e%3c/svg%3e");
  background-size: 14px 14px;
  background-repeat: no-repeat;
  background-position: center;
  opacity: 0.5;
  z-index: 1;
}

/* Error state */
:deep(.vue-tel-input.error) {
  border-color: hsl(var(--destructive));
  --tw-ring-color: hsl(var(--destructive));
}

/* Absolute fallback: force compact sizing globally for this component instance.
   This avoids any scoped/:deep edge-cases with 3rd-party rendered DOM. */
:global(.tel-input.vue-tel-input .vti__dropdown-list) {
  font-size: 14px !important;
}

:global(.tel-input.vue-tel-input .vti__dropdown-item) {
  padding: 2px 8px !important;
  min-height: 24px !important;
  line-height: 1.1 !important;
  font-size: 14px !important;
}

:global(.tel-input.vue-tel-input .vti__dropdown-item strong) {
  font-size: inherit !important;
  font-weight: 500 !important;
}

:global(.tel-input.vue-tel-input .vti__search_box) {
  height: 28px !important;
  font-size: 14px !important;
}

/* Search box container (the library wraps the input in this div inside the <ul>) */
:global(.tel-input.vue-tel-input .vti__search_box_container) {
  position: relative !important;
  padding: 4px 6px !important;
  border-bottom: 1px solid hsl(var(--border)) !important;
  background: hsl(var(--background)) !important;
}

:global(.tel-input.vue-tel-input .vti__search_box_container::before) {
  content: '' !important;
  position: absolute !important;
  left: 8px !important;
  top: 50% !important;
  transform: translateY(-50%) !important;
  width: 14px !important;
  height: 14px !important;
  opacity: 0.55 !important;
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke-width='1.5' stroke='currentColor'%3e%3cpath stroke-linecap='round' stroke-linejoin='round' d='m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z' /%3e%3c/svg%3e");
  background-size: 14px 14px !important;
  background-repeat: no-repeat !important;
  background-position: center !important;
  pointer-events: none !important;
}

:global(.tel-input.vue-tel-input .vti__search_box_container .vti__search_box) {
  display: block !important;
  width: 100% !important;
  height: 28px !important;
  padding: 0 8px 0 28px !important;
  border: 0 !important;
  outline: 0 !important;
  background: transparent !important;
  font-size: 14px !important;
  line-height: 1.1 !important;
}

/* Slim scrollbar for dropdown */
:global(.tel-input.vue-tel-input .vti__dropdown-list) {
  scrollbar-width: thin;
  scrollbar-color: hsl(var(--border)) transparent;
}

:global(.tel-input.vue-tel-input .vti__dropdown-list::-webkit-scrollbar) {
  width: 8px;
}

:global(.tel-input.vue-tel-input .vti__dropdown-list::-webkit-scrollbar-track) {
  background: transparent;
}

:global(.tel-input.vue-tel-input .vti__dropdown-list::-webkit-scrollbar-thumb) {
  background-color: hsl(var(--border));
  border-radius: 9999px;
  border: 2px solid transparent;
  background-clip: content-box;
}
</style>

 