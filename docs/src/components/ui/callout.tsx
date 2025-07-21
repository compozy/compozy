import {
  Alert,
  AlertContent,
  AlertDescription,
  AlertIcon,
  AlertTitle,
} from "@/components/ui/alert";
import { AlertTriangle, CheckCircle, Info, XCircle } from "lucide-react";
import { Icon } from "@/components/ui/icon";

type CalloutType = "info" | "warning" | "success" | "error" | "neutral";

interface CalloutProps extends Omit<React.ComponentProps<typeof Alert>, "variant" | "icon"> {
  type?: CalloutType;
  icon?: React.ReactNode | string;
  title?: string;
}

const typeToVariantMap: Record<CalloutType, React.ComponentProps<typeof Alert>["variant"]> = {
  info: "info",
  warning: "warning",
  success: "success",
  error: "destructive",
  neutral: "secondary",
};

const typeToIconMap: Record<CalloutType, React.ReactNode> = {
  info: <Info className="size-3" />,
  warning: <AlertTriangle className="size-3" />,
  success: <CheckCircle className="size-3" />,
  error: <XCircle className="size-3" />,
  neutral: <Info className="size-3" />,
};

export function Callout({
  title,
  children,
  icon,
  type = "neutral",
  className: _className,
  ...props
}: CalloutProps) {
  const variant = typeToVariantMap[type];
  const defaultIcon = typeToIconMap[type];

  return (
    <Alert appearance="outline" variant={variant} {...props} className="not-prose">
      <AlertIcon>
        {typeof icon === "string" ? (
          <Icon name={icon} className="size-3" />
        ) : (
          icon || defaultIcon
        )}
      </AlertIcon>
      <AlertContent>
        {title && <AlertTitle>{title}</AlertTitle>}
        <AlertDescription className="text-muted-foreground [&>p]:mb-0">{children}</AlertDescription>
      </AlertContent>
    </Alert>
  );
}
