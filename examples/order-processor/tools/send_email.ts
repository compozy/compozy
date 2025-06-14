#!/usr/bin/env -S deno run --allow-read --allow-net

/**
 * Email Sender Tool
 *
 * Sends email notifications to customers. This is a mock implementation
 * that logs email content instead of actually sending emails.
 */

interface EmailInput {
  recipient: string;
  subject: string;
  template: string;
  data: Record<string, any>;
}

interface EmailOutput {
  sent: boolean;
  messageId: string;
  recipient: string;
  subject: string;
  timestamp: string;
}

// Email templates
const templates = {
  order_confirmation: (data: any) => `
Dear Customer,

Thank you for your order! We're excited to confirm that your order has been successfully processed.

Order Details:
- Order ID: ${data.order_id}
- Items: ${data.items?.length || 0} items
- Subtotal: $${data.subtotal}
- Shipping: $${data.shipping_cost}
- Total: $${data.total}
- Estimated Delivery: ${data.estimated_delivery}

${data.confirmation_message || ''}

We'll send you tracking information once your order ships.

Best regards,
The Order Processing Team
`,

  validation_error: (data: any) => `
Dear Customer,

We encountered an issue while processing your order ${data.order_id}.

Error Details: ${data.error_details}

Our customer service team has been notified and will contact you shortly to resolve this issue.

We apologize for any inconvenience.

Best regards,
Customer Service Team
`,

  inventory_error: (data: any) => `
Dear Customer,

We're sorry to inform you that some items in your order ${data.order_id} are currently out of stock.

We're working to restock these items and will update you on availability within 24 hours.

If you'd like to proceed with available items only or cancel your order, please contact our customer service team.

Best regards,
Inventory Management Team
`,

  shipping_error: (data: any) => `
Dear Customer,

We encountered an issue calculating shipping for your order ${data.order_id}.

Shipping Address:
${data.shipping_address?.street}
${data.shipping_address?.city}, ${data.shipping_address?.zipCode}
${data.shipping_address?.country}

Our shipping team will contact you within 24 hours to resolve this and provide accurate shipping information.

Best regards,
Shipping Department
`,

  payment_error: (data: any) => `
Dear Customer,

We were unable to process payment for your order ${data.order_id}.

Amount: $${data.amount}
Error: ${data.error_details}

Please check your payment method or contact your bank. You can retry payment by visiting your account or contacting our customer service team.

Best regards,
Payment Processing Team
`
};

async function sendEmail(input: EmailInput): Promise<EmailOutput> {
  // Generate a mock message ID
  const messageId = `msg_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

  // Get template and render with data
  const templateFn = templates[input.template as keyof typeof templates];
  if (!templateFn) {
    throw new Error(`Unknown email template: ${input.template}`);
  }

  const emailBody = templateFn(input.data);

  // Mock email sending - in production, integrate with actual email service
  console.log(`
🔔 EMAIL NOTIFICATION
---
To: ${input.recipient}
Subject: ${input.subject}
Message ID: ${messageId}
Template: ${input.template}
---
${emailBody}
---
✅ Email sent successfully
`);

  // Simulate network delay
  await new Promise(resolve => setTimeout(resolve, 100));

  return {
    sent: true,
    messageId,
    recipient: input.recipient,
    subject: input.subject,
    timestamp: new Date().toISOString()
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

    const input: EmailInput = JSON.parse(inputText);
    const result = await sendEmail(input);

    console.log(JSON.stringify(result, null, 2));
  } catch (error) {
    console.error(JSON.stringify({
      error: true,
      message: error.message,
      type: "email_send_error"
    }));
    Deno.exit(1);
  }
}
