#!/usr/bin/env -S deno run --allow-read --allow-net

/**
 * Inventory Checker Tool
 *
 * Checks product inventory levels to ensure order can be fulfilled.
 * This is a mock implementation for demonstration purposes.
 */

interface InventoryInput {
  items: Array<{
    productId: string;
    name: string;
    quantity: number;
    price: number;
  }>;
}

interface InventoryOutput {
  available: boolean;
  itemStatus: Array<{
    productId: string;
    name: string;
    requested: number;
    available: number;
    sufficient: boolean;
  }>;
  warnings: string[];
}

async function checkInventory(input: InventoryInput): Promise<InventoryOutput> {
  // Mock inventory database
  const inventory = new Map([
    ["prod_001", 25],
    ["prod_002", 10],
    ["prod_003", 0],
    ["prod_004", 100],
    ["prod_005", 5],
  ]);

  const itemStatus = input.items.map(item => {
    const availableStock = inventory.get(item.productId) ?? 0;
    const sufficient = availableStock >= item.quantity;

    return {
      productId: item.productId,
      name: item.name,
      requested: item.quantity,
      available: availableStock,
      sufficient
    };
  });

  const allAvailable = itemStatus.every(item => item.sufficient);
  const warnings: string[] = [];

  // Generate warnings for low stock or out of stock items
  itemStatus.forEach(item => {
    if (!item.sufficient) {
      warnings.push(`Insufficient stock for ${item.name}: requested ${item.requested}, available ${item.available}`);
    } else if (item.available <= item.requested + 2) {
      warnings.push(`Low stock warning for ${item.name}: only ${item.available} remaining after order`);
    }
  });

  return {
    available: allAvailable,
    itemStatus,
    warnings
  };
}

// Main execution
if (import.meta.main) {
  try {
    const inputText = await new Promise<string>((resolve) => {
      const chunks: string[] = [];
      const decoder = new TextDecoder();

      const reader = Deno.stdin.readable.getReader();

      const pump = async (): Promise<void> => {
        const { done, value } = await reader.read();
        if (done) {
          resolve(chunks.join(''));
          return;
        }
        chunks.push(decoder.decode(value));
        return pump();
      };

      pump();
    });

    const input: InventoryInput = JSON.parse(inputText);
    const result = await checkInventory(input);

    console.log(JSON.stringify(result, null, 2));
  } catch (error) {
    console.error(JSON.stringify({
      error: true,
      message: error.message,
      type: "inventory_check_error"
    }));
    Deno.exit(1);
  }
}
