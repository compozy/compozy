"use client";

import { Tabs as ReuiTabs, TabsContent, TabsList, TabsTrigger } from "@/components/reui/tabs";
import { cn } from "@/lib/utils";
import * as React from "react";

interface TabsProps {
  items: string[];
  children: React.ReactNode;
  className?: string;
  defaultValue?: string;
  updateAnchor?: boolean;
  defaultIndex?: number;
}

interface TabProps {
  id?: string;
  value?: string;
  children: React.ReactNode;
  className?: string;
}

// Helper function to escape values (convert spaces to hyphens)
function escapeValue(v: string): string {
  return v.toLowerCase().replace(/\s+/g, "-");
}

// Context to track tab collection
const TabContext = React.createContext<{
  items: string[];
  collection: symbol[];
}>({ items: [], collection: [] });

function useTabContext() {
  const ctx = React.useContext(TabContext);
  if (!ctx) throw new Error("You must wrap your component in <Tabs>");
  return ctx;
}

export function Tabs({
  items,
  children,
  className,
  defaultValue,
  updateAnchor: _updateAnchor = false,
  defaultIndex = 0,
}: TabsProps) {
  const collection = React.useMemo<symbol[]>(() => [], []);

  // Use the defaultValue or first item
  const initialValue = defaultValue || items[defaultIndex];

  return (
    <ReuiTabs defaultValue={escapeValue(initialValue)} className={cn("w-full", className)}>
      <TabsList variant="line" size="md" className="w-full">
        {items.map(item => (
          <TabsTrigger key={item} value={escapeValue(item)}>
            {item}
          </TabsTrigger>
        ))}
      </TabsList>
      <TabContext.Provider value={{ items, collection }}>{children}</TabContext.Provider>
    </ReuiTabs>
  );
}

// Hook to get the index of the current Tab component
function useCollectionIndex() {
  const key = React.useId();
  const { collection } = useTabContext();

  React.useEffect(() => {
    return () => {
      const idx = collection.indexOf(key as any);
      if (idx !== -1) collection.splice(idx, 1);
    };
  }, [key, collection]);

  if (!collection.includes(key as any)) collection.push(key as any);
  return collection.indexOf(key as any);
}

export function Tab({ children, className, value }: TabProps) {
  const { items } = useTabContext();
  const index = useCollectionIndex();

  // Resolve the value from props or index
  const resolved = value ?? items?.at(index);

  if (!resolved) {
    throw new Error(
      "Failed to resolve tab `value`, please pass a `value` prop to the Tab component."
    );
  }

  return (
    <TabsContent value={escapeValue(resolved)} className={cn("mt-6", className)}>
      {children}
    </TabsContent>
  );
}
