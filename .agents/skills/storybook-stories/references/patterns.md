# Storybook Patterns Reference

## Accordion Pattern (Compound Component)

Compound components require a `render` function to structure the children correctly.

```tsx
export const Default: Story = {
  render: () => (
    <Accordion>
      <AccordionItem value="item-1">
        <AccordionTrigger>Is it accessible?</AccordionTrigger>
        <AccordionContent>Yes. It adheres to the WAI-ARIA design pattern.</AccordionContent>
      </AccordionItem>
      {/* More items... */}
    </Accordion>
  ),
};
```

## Simple Component Pattern (Props-based)

Simple components can often just use `args`.

```tsx
export const Default: Story = {
  args: {
    variant: "default",
    size: "md",
    children: "Button Text",
  },
};
```

## Wrapped Component Pattern

Sometimes a component needs a wrapper to look right in the centered layout (e.g., a Toast or a large Card).

```tsx
decorators: [
  (Story) => (
    <div className="w-[400px] p-4 border rounded">
      <Story />
    </div>
  ),
],
```

## Interactive/Stateful Pattern

For components that might need internal state control in stories (though Storybook controls often handle this), you can use standard React hooks inside the render function if absolutely necessary, but usually, uncontrolled primitives are preferred for display.

```tsx
export const Interactive: Story = {
  render: () => {
    // ... implementation
    return <Component />;
  },
};
```
