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

export default function run(input: InventoryInput): InventoryOutput {
    // Mock inventory database
    const inventory = new Map([
        ["prod_001", 25],
        ["prod_002", 10],
        ["prod_003", 0],
        ["prod_004", 100],
        ["prod_005", 5],
    ]);

    const itemStatus = input.items.map((item) => {
        const availableStock = inventory.get(item.productId) ?? 0;
        const sufficient = availableStock >= item.quantity;

        return {
            productId: item.productId,
            name: item.name,
            requested: item.quantity,
            available: availableStock,
            sufficient,
        };
    });

    const allAvailable = itemStatus.every((item) => item.sufficient);
    const warnings: string[] = [];

    // Generate warnings for low stock or out of stock items
    itemStatus.forEach((item) => {
        if (!item.sufficient) {
            warnings.push(
                `Insufficient stock for ${item.name}: requested ${item.requested}, available ${item.available}`,
            );
        } else if (item.available <= item.requested + 2) {
            warnings.push(
                `Low stock warning for ${item.name}: only ${item.available} remaining after order`,
            );
        }
    });



    return {
        available: allAvailable,
        itemStatus,
        warnings,
    };
}

// Alternative export for compatibility
export { run };
