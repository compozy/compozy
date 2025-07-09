"use client";

import { cn } from "@/lib/utils";
import { Check, Circle } from "lucide-react";
import { motion, useScroll } from "motion/react";
import React, { createContext, useContext, useEffect, useRef, useState } from "react";
import { tv, type VariantProps } from "tailwind-variants";

// Step component using tailwind-variants
const stepVariants = tv({
  slots: {
    base: "relative grid grid-cols-[auto_1fr]",
    left: "flex flex-col items-center",
    indicator: [
      "relative flex items-center justify-center",
      "font-medium transition-all duration-200",
      "rounded-full flex-shrink-0 border-2",
      // Force minimum sizes to prevent collapse
      "min-w-[2rem] min-h-[2rem]",
    ],
    connector: "relative w-px h-full transition-all duration-300",
    content: "flex flex-col ml-4 pb-8",
    title: "transition-colors duration-200 mt-0 font-semibold",
    description: "text-sm text-muted-foreground mt-1",
  },
  variants: {
    size: {
      sm: {
        indicator: ["w-8 h-8 !min-w-[2rem] !min-h-[2rem]", "text-xs"],
        connector: "",
        content: "ml-3",
        title: "text-sm",
        description: "text-xs",
      },
      md: {
        indicator: ["w-12 h-12 !min-w-[2.5rem] !min-h-[2.5rem]", "text-sm"],
        connector: "",
        content: "ml-4",
        title: "text-base",
        description: "text-sm",
      },
      lg: {
        indicator: ["w-14 h-14 !min-w-[3rem] !min-h-[3rem]", "text-base"],
        connector: "",
        content: "ml-8",
        title: "text-lg",
        description: "text-base",
      },
    },
    state: {
      active: {
        indicator: ["bg-primary text-primary-foreground", "border-primary shadow-sm"],
        title: "text-primary",
      },
      completed: {
        indicator: "bg-primary text-primary-foreground border-primary",
        title: "text-foreground",
      },
      upcoming: {
        indicator: "bg-background text-muted-foreground border-border",
        title: "text-foreground",
      },
      error: {
        indicator: "bg-destructive/10 text-destructive border-destructive",
        title: "text-destructive",
      },
    },
  },
  defaultVariants: {
    size: "md",
    state: "upcoming",
  },
});

// Steps container using tailwind-variants
const stepsVariants = tv({
  base: "my-14 flex flex-col w-full",
});

// Types for the step variants
type StepVariants = VariantProps<typeof stepVariants>;

// Title component type
type TitleComponent = "h1" | "h2" | "h3" | "h4" | "h5" | "h6" | "div" | "span" | "p";

// Steps context
interface StepsContextValue {
  titleComponent?: TitleComponent;
  size?: "sm" | "md" | "lg";
}

const StepsContext = createContext<StepsContextValue | undefined>(undefined);

const useStepsContext = () => {
  const context = useContext(StepsContext);
  return context;
};

// Custom hook for scroll-based step animation
function useStepScrollAnimation(
  stepRef: React.RefObject<HTMLDivElement | null>,
  offset: number = 400
) {
  const [isInView, setIsInView] = useState(false);
  const [connectorProgress, setConnectorProgress] = useState(0);
  const { scrollY } = useScroll();

  useEffect(() => {
    const updateProgress = () => {
      if (!stepRef.current) return;

      // Get the total page height and current scroll position
      const totalPageHeight = document.documentElement.scrollHeight - window.innerHeight;
      const currentScrollY = window.scrollY;

      // Calculate overall page progress (0 to 1)
      const pageProgress = totalPageHeight > 0 ? currentScrollY / totalPageHeight : 0;

      // Get element position on the page
      const elementTop = stepRef.current.offsetTop;
      const elementHeight = stepRef.current.offsetHeight;

      // Calculate when this step should be considered "active" based on page progress
      // Each step becomes active with specified offset before it would normally enter
      const stepProgressThreshold = Math.max(0, (elementTop - offset) / totalPageHeight);
      const nextStepThreshold = Math.max(
        0,
        (elementTop + elementHeight - offset) / totalPageHeight
      );

      // Set in view when page scroll reaches this step's threshold
      setIsInView(pageProgress >= stepProgressThreshold);

      // Calculate connector progress based on page scroll between this step and next
      const connectorStart = stepProgressThreshold;
      const connectorEnd = nextStepThreshold;
      const connectorRange = connectorEnd - connectorStart;

      let progress = 0;
      if (connectorRange > 0) {
        progress = Math.max(0, Math.min(1, (pageProgress - connectorStart) / connectorRange));
      }

      setConnectorProgress(progress);
    };

    const unsubscribe = scrollY.on("change", updateProgress);

    // Initial calculation
    updateProgress();

    return () => unsubscribe();
  }, [scrollY, offset]);

  return { isInView, connectorProgress };
}

// Step Indicator Component
interface StepIndicatorProps {
  state: "active" | "completed" | "upcoming" | "error";
  size: "sm" | "md" | "lg";
  isInView: boolean;
  connectorProgress: number;
  icon?: React.ReactNode;
  stepNumber?: number;
  className?: string;
}

function StepIndicator({
  state,
  size,
  isInView,
  connectorProgress,
  icon,
  stepNumber,
  className,
}: StepIndicatorProps) {
  const styles = stepVariants({ size, state });

  // Icon size based on size variant
  const iconSizeClasses = {
    sm: "w-4 h-4",
    md: "w-5 h-5",
    lg: "w-6 h-6",
  };

  const renderIcon = () => {
    if (state === "completed" || (isInView && connectorProgress > 0.8)) {
      return <Check className={iconSizeClasses[size]} />;
    }

    if (state === "error") {
      return <Circle className={cn(iconSizeClasses[size], "fill-current")} />;
    }

    if (icon) {
      return React.isValidElement(icon)
        ? React.cloneElement(icon as React.ReactElement<any>, {
            className: cn(iconSizeClasses[size], (icon as React.ReactElement<any>).props.className),
          })
        : icon;
    }

    return stepNumber;
  };

  return (
    <motion.div
      className={cn(styles.indicator(), className)}
      animate={{
        scale: isInView ? 1.05 : 1,
        opacity: isInView ? 1 : 0.7,
      }}
      transition={{ duration: 0.5, ease: "easeOut" }}
    >
      {renderIcon()}
    </motion.div>
  );
}

// Step Connector Component
interface StepConnectorProps {
  connectorProgress: number;
  size: "sm" | "md" | "lg";
  className?: string;
}

function StepConnector({ connectorProgress, size, className }: StepConnectorProps) {
  const styles = stepVariants({ size });

  return (
    <div className={cn(styles.connector(), "relative overflow-hidden", className)}>
      {/* Base connector line */}
      <div className="absolute inset-0 bg-border" />
      {/* Animated gradient overlay */}
      <motion.div
        className="absolute inset-x-0 top-0 w-full bg-gradient-to-b from-primary to-primary/20"
        style={{
          height: `${connectorProgress * 100}%`,
        }}
      />
    </div>
  );
}

// Step Content Component
interface StepContentProps {
  title?: string;
  description?: string;
  children?: React.ReactNode;
  titleComponent?: TitleComponent;
  size: "sm" | "md" | "lg";
  state: "active" | "completed" | "upcoming" | "error";
  isInView: boolean;
  className?: string;
}

function StepContent({
  title,
  description,
  children,
  titleComponent,
  size,
  state,
  isInView,
  className,
}: StepContentProps) {
  const stepsContext = useStepsContext();
  const styles = stepVariants({ size, state });

  // Determine which title component to use (individual overrides global)
  const TitleTag = (titleComponent || stepsContext?.titleComponent || "div") as React.ElementType;

  return (
    <motion.div
      className={cn(styles.content(), className)}
      animate={{
        opacity: isInView ? 1 : 0.6,
        x: isInView ? 0 : -10,
      }}
      transition={{ duration: 0.3 }}
    >
      {title && <TitleTag className={styles.title()}>{title}</TitleTag>}
      {description && <div className={styles.description()}>{description}</div>}
      {children}
    </motion.div>
  );
}

export interface StepProps extends StepVariants {
  title?: string;
  titleComponent?: TitleComponent;
  description?: string;
  icon?: React.ReactNode;
  isLast?: boolean;
  stepNumber?: number;
  children?: React.ReactNode;
  className?: string;
  scrollOffset?: number;
}

export interface StepsProps {
  children: React.ReactNode;
  currentStep?: number;
  className?: string;
  size?: "sm" | "md" | "lg";
  titleComponent?: TitleComponent;
}

// Individual Step component
export function Step({
  className,
  size,
  state,
  title,
  titleComponent,
  description,
  icon,
  isLast,
  stepNumber,
  children,
  scrollOffset = 400,
}: StepProps) {
  const stepsContext = useStepsContext();
  const stepRef = useRef<HTMLDivElement>(null);

  // Use custom hook for scroll animation
  const { isInView, connectorProgress } = useStepScrollAnimation(stepRef, scrollOffset);

  // Use context values with local prop fallbacks
  const currentSize = size || stepsContext?.size || "md";
  const currentState = isInView ? "active" : state || "upcoming";

  const styles = stepVariants({
    size: currentSize,
    state: currentState,
  });

  return (
    <div ref={stepRef} className={cn(styles.base(), className)}>
      <div className={styles.left()}>
        {/* Step Indicator */}
        <StepIndicator
          state={currentState}
          size={currentSize}
          isInView={isInView}
          connectorProgress={connectorProgress}
          icon={icon}
          stepNumber={stepNumber}
        />

        {/* Connector Line */}
        {!isLast && <StepConnector connectorProgress={connectorProgress} size={currentSize} />}
      </div>

      {/* Content */}
      <StepContent
        title={title}
        description={description}
        titleComponent={titleComponent}
        size={currentSize}
        state={currentState}
        isInView={isInView}
      >
        {children}
      </StepContent>
    </div>
  );
}

// Steps container component
export function Steps({
  className,
  currentStep = 0,
  children,
  size = "md",
  titleComponent,
}: StepsProps) {
  const styles = stepsVariants();
  const totalSteps = React.Children.count(children);

  const contextValue: StepsContextValue = {
    titleComponent,
    size,
  };

  const steps = React.Children.toArray(children).map((child, index) => {
    if (React.isValidElement<StepProps>(child) && child.type === Step) {
      const isCompleted = currentStep > index;
      const isActive = currentStep === index;
      const isLast = index === totalSteps - 1;

      return React.cloneElement(child, {
        ...child.props,
        state: child.props.state || (isCompleted ? "completed" : isActive ? "active" : "upcoming"),
        isLast,
        stepNumber: index + 1,
      });
    }
    return child;
  });

  return (
    <StepsContext.Provider value={contextValue}>
      <div className={cn(styles, className)}>{steps}</div>
    </StepsContext.Provider>
  );
}
