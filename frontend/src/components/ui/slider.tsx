"use client"

import * as React from "react"
import { Slider as SliderPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

/** Slider is the shadcn range slider: one thumb per value, with an optional label per thumb for screen readers. */
function Slider({
  className,
  defaultValue,
  value,
  min = 0,
  max = 100,
  thumbLabels,
  ...props
}: React.ComponentProps<typeof SliderPrimitive.Root> & { thumbLabels?: string[] }) {
  const values = React.useMemo(
    () => (Array.isArray(value) ? value : Array.isArray(defaultValue) ? defaultValue : [min, max]),
    [value, defaultValue, min, max]
  )

  return (
    <SliderPrimitive.Root
      data-slot="slider"
      defaultValue={defaultValue}
      value={value}
      min={min}
      max={max}
      className={cn(
        "relative flex w-full touch-none items-center select-none data-[disabled]:opacity-50",
        className
      )}
      {...props}
    >
      <SliderPrimitive.Track
        data-slot="slider-track"
        className="relative h-[2px] w-full grow overflow-hidden rounded-full bg-[var(--border)]"
      >
        <SliderPrimitive.Range data-slot="slider-range" className="absolute h-full bg-[var(--foreground)]" />
      </SliderPrimitive.Track>
      {values.map((_, index) => (
        <SliderPrimitive.Thumb
          data-slot="slider-thumb"
          key={index}
          aria-label={thumbLabels?.[index]}
          className="block size-5 shrink-0 rounded-full border-2 border-[var(--foreground)] bg-[var(--background)] shadow-sm transition-[box-shadow] hover:ring-4 hover:ring-[var(--border)] focus-visible:ring-4 focus-visible:ring-[var(--border)] focus-visible:outline-hidden"
        />
      ))}
    </SliderPrimitive.Root>
  )
}

export { Slider }
