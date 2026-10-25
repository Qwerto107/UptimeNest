export interface SelectOption<T extends string | number = string | number> {
  value: T
  label: string
  disabled?: boolean
}

export interface SelectOptionGroup<T extends string | number = string | number> {
  label?: string
  options: readonly SelectOption<T>[]
}
