import type { Meta } from "@storybook/nextjs-vite";
import {
  Code,
  Database,
  FileText,
  GitBranch,
  Package,
  Rocket,
  Search,
  Shield,
  Terminal,
  Zap,
} from "lucide-react";
import { Step, Steps } from "./step";

const meta = {
  title: "UI/Step",
  component: Steps,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof Steps>;

export default meta;

// Basic vertical steps
export const Default = {
  render: () => (
    <div className="max-w-md">
      <Steps currentStep={1}>
        <Step title="Account Setup" description="Create your account and profile" />
        <Step title="Preferences" description="Configure your settings" />
        <Step title="Complete" description="Review and finish" />
      </Steps>
    </div>
  ),
};

// Steps with numbers instead of icons
export const Numbered = {
  render: () => (
    <div className="max-w-md">
      <Steps currentStep={2} numbered>
        <Step title="First Step" description="This is the first step with a number" />
        <Step title="Second Step" description="This is the second step with a number" />
        <Step title="Third Step" description="This is the third step with a number" />
        <Step title="Fourth Step" description="This is the fourth step with a number" />
      </Steps>
    </div>
  ),
};

// Steps with icons
export const WithIcons = {
  render: () => (
    <div className="max-w-md">
      <Steps currentStep={2}>
        <Step
          icon={<Shield className="size-5" />}
          title="Secure Account"
          description="Enable two-factor authentication and security features"
        />
        <Step
          icon={<Zap className="size-5" />}
          title="Optimize Performance"
          description="Configure caching and performance settings"
        />
        <Step
          icon={<Rocket className="size-5" />}
          title="Deploy Application"
          description="Push your application to production environment"
        />
        <Step
          icon={<Database className="size-5" />}
          title="Verify Deployment"
          description="Check that all services are running correctly"
        />
      </Steps>
    </div>
  ),
};

// Different sizes
export const Sizes = {
  render: () => (
    <div className="space-y-12">
      <div>
        <h3 className="text-sm font-medium mb-4">Small</h3>
        <div className="max-w-md">
          <Steps currentStep={1} size="sm">
            <Step title="Initialize" description="Set up project structure" />
            <Step title="Configure" description="Add dependencies" />
            <Step title="Build" description="Compile the application" />
          </Steps>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium mb-4">Medium (Default)</h3>
        <div className="max-w-md">
          <Steps currentStep={1} size="md">
            <Step title="Initialize" description="Set up project structure" />
            <Step title="Configure" description="Add dependencies" />
            <Step title="Build" description="Compile the application" />
          </Steps>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium mb-4">Large</h3>
        <div className="max-w-md">
          <Steps currentStep={1} size="lg">
            <Step title="Initialize" description="Set up project structure" />
            <Step title="Configure" description="Add dependencies" />
            <Step title="Build" description="Compile the application" />
          </Steps>
        </div>
      </div>
    </div>
  ),
};

// Different sizes with numbers
export const NumberedSizes = {
  render: () => (
    <div className="space-y-12">
      <div>
        <h3 className="text-sm font-medium mb-4">Small Numbered Steps</h3>
        <div className="max-w-md">
          <Steps currentStep={1} size="sm" numbered>
            <Step title="Initialize" description="Set up project structure" />
            <Step title="Configure" description="Add dependencies" />
            <Step title="Build" description="Compile the application" />
          </Steps>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium mb-4">Medium Numbered Steps</h3>
        <div className="max-w-md">
          <Steps currentStep={1} size="md" numbered>
            <Step title="Initialize" description="Set up project structure" />
            <Step title="Configure" description="Add dependencies" />
            <Step title="Build" description="Compile the application" />
          </Steps>
        </div>
      </div>

      <div>
        <h3 className="text-sm font-medium mb-4">Large Numbered Steps</h3>
        <div className="max-w-md">
          <Steps currentStep={1} size="lg" numbered>
            <Step title="Initialize" description="Set up project structure" />
            <Step title="Configure" description="Add dependencies" />
            <Step title="Build" description="Compile the application" />
          </Steps>
        </div>
      </div>
    </div>
  ),
};

// With error state
export const WithError = {
  render: () => (
    <div className="max-w-md">
      <Steps currentStep={2}>
        <Step title="Upload Files" description="Select and upload your documents" />
        <Step title="Validate Data" description="Check for errors and inconsistencies" />
        <Step title="Process Files" description="Transform and analyze your data" state="error" />
        <Step title="Complete" description="Download results" />
      </Steps>
    </div>
  ),
};

// Installation guide example
// Steps with scroll-based animations
export const ScrollAnimated = {
  render: () => (
    <div className="max-w-2xl">
      <div className="mb-8 p-4 bg-muted rounded-lg">
        <p className="text-sm text-muted-foreground">
          Scroll down to see the steps animate based on your scroll position. The indicators will
          highlight and connectors will fill with a gradient as you scroll.
        </p>
      </div>

      <div className="space-y-32">
        <Steps>
          <Step
            icon={<Search className="size-5" />}
            title="Discovery Phase"
            description="Research and gather requirements for your project"
          >
            <div className="mt-4 p-4 bg-muted rounded-lg">
              <p className="text-sm">Conduct user interviews and market analysis</p>
            </div>
          </Step>

          <Step
            icon={<Code className="size-5" />}
            title="Development"
            description="Build your application with modern tools"
          >
            <div className="mt-4 p-4 bg-muted rounded-lg">
              <p className="text-sm">Implement features using best practices</p>
            </div>
          </Step>

          <Step
            icon={<Database className="size-5" />}
            title="Data Migration"
            description="Transfer and validate your data"
          >
            <div className="mt-4 p-4 bg-muted rounded-lg">
              <p className="text-sm">Ensure data integrity throughout the process</p>
            </div>
          </Step>

          <Step
            icon={<Package className="size-5" />}
            title="Deployment"
            description="Package and deploy your application"
          >
            <div className="mt-4 p-4 bg-muted rounded-lg">
              <p className="text-sm">Configure CI/CD pipelines for automated deployment</p>
            </div>
          </Step>

          <Step
            icon={<Rocket className="size-5" />}
            title="Launch"
            description="Go live and monitor performance"
          >
            <div className="mt-4 p-4 bg-muted rounded-lg">
              <p className="text-sm">Track metrics and gather user feedback</p>
            </div>
          </Step>
        </Steps>
      </div>

      <div className="h-96" />
    </div>
  ),
  parameters: {
    layout: "fullscreen",
  },
};

export const InstallationGuide = {
  render: () => (
    <div className="max-w-2xl">
      <h2 className="text-2xl font-bold mb-6">Getting Started</h2>
      <Steps currentStep={3}>
        <Step
          icon={<Terminal className="size-5" />}
          title="Install Dependencies"
          description="Run the package manager to install required dependencies"
        >
          <div className="mt-4 p-4 bg-muted rounded-lg">
            <code className="text-sm">bun install</code>
          </div>
        </Step>

        <Step
          icon={<FileText className="size-5" />}
          title="Configure Environment"
          description="Set up your environment variables"
        >
          <div className="mt-4 p-4 bg-muted rounded-lg">
            <code className="text-sm">cp .env.example .env</code>
          </div>
        </Step>

        <Step
          icon={<GitBranch className="size-5" />}
          title="Initialize Repository"
          description="Set up version control for your project"
        >
          <div className="mt-4 p-4 bg-muted rounded-lg">
            <code className="text-sm">git init</code>
          </div>
        </Step>

        <Step
          icon={<Rocket className="size-5" />}
          title="Start Development"
          description="Launch the development server"
        >
          <div className="mt-4 p-4 bg-muted rounded-lg">
            <code className="text-sm">bun run dev</code>
          </div>
        </Step>
      </Steps>
    </div>
  ),
};

// API Integration workflow
export const APIWorkflow = {
  render: () => (
    <div className="max-w-xl">
      <h2 className="text-2xl font-bold mb-6">API Integration Process</h2>
      <Steps currentStep={0}>
        <Step
          icon={<Code className="size-5" />}
          title="Define Schema"
          description="Create TypeScript interfaces for your API endpoints"
        />

        <Step
          icon={<Search className="size-5" />}
          title="Implement Endpoints"
          description="Build RESTful API routes with proper validation"
        />

        <Step
          icon={<Database className="size-5" />}
          title="Connect Database"
          description="Set up database connections and migrations"
        />

        <Step
          icon={<Shield className="size-5" />}
          title="Add Authentication"
          description="Implement JWT tokens and secure routes"
        />

        <Step
          icon={<Package className="size-5" />}
          title="Deploy to Production"
          description="Configure CI/CD pipeline and deploy"
        />
      </Steps>
    </div>
  ),
};

// All states example
export const AllStates = {
  render: () => (
    <div className="max-w-md">
      <h2 className="text-2xl font-bold mb-6">Step States</h2>
      <Steps>
        <Step title="Completed Step" description="This step has been completed" state="completed" />
        <Step title="Active Step" description="Currently working on this step" state="active" />
        <Step title="Error Step" description="This step encountered an error" state="error" />
        <Step title="Upcoming Step" description="This step is pending" state="upcoming" />
      </Steps>
    </div>
  ),
};

// Title components - Global
export const GlobalTitleComponent = {
  render: () => (
    <div className="max-w-md space-y-8">
      <div>
        <h3 className="text-lg font-semibold mb-4">Using H3 for all titles</h3>
        <Steps currentStep={1} titleComponent="h3">
          <Step title="First Step" description="This uses an H3 tag from the global setting" />
          <Step title="Second Step" description="Also using H3 from global" />
          <Step title="Third Step" description="All titles are H3 elements" />
        </Steps>
      </div>

      <div>
        <h3 className="text-lg font-semibold mb-4">Using H4 for all titles</h3>
        <Steps currentStep={2} titleComponent="h4" size="lg">
          <Step icon={<FileText />} title="Documentation" description="Write comprehensive docs" />
          <Step icon={<Code />} title="Implementation" description="Build the features" />
          <Step icon={<Rocket />} title="Deployment" description="Ship to production" />
        </Steps>
      </div>
    </div>
  ),
};

// Title components - Individual overrides
export const IndividualTitleComponents = {
  render: () => (
    <div className="max-w-md">
      <h2 className="text-2xl font-bold mb-6">Mixed Title Components</h2>
      <Steps currentStep={1} titleComponent="h3">
        <Step title="Overview" description="This uses the global H3 setting" />
        <Step
          title="Important Section"
          titleComponent="h2"
          description="This overrides with H2 for emphasis"
        />
        <Step
          title="Sub-section"
          titleComponent="h4"
          description="This overrides with H4 for hierarchy"
        />
        <Step
          title="Details"
          titleComponent="h5"
          description="This overrides with H5 for minor headings"
        />
      </Steps>
    </div>
  ),
};

// Semantic HTML structure
export const SemanticStructure = {
  render: () => (
    <div className="max-w-2xl">
      <h1 className="text-3xl font-bold mb-2">Installation Guide</h1>
      <p className="text-muted-foreground mb-8">
        Follow these steps to set up your development environment
      </p>

      <Steps currentStep={0} titleComponent="h2" size="lg">
        <Step
          icon={<Package />}
          title="Prerequisites"
          description="Ensure you have the required tools installed"
        >
          <div className="mt-4 space-y-2">
            <p className="text-sm">Required:</p>
            <ul className="text-sm list-disc list-inside text-muted-foreground">
              <li>Node.js 18+ or Bun</li>
              <li>Git version control</li>
              <li>Code editor (VS Code recommended)</li>
            </ul>
          </div>
        </Step>

        <Step
          icon={<Terminal />}
          title="Installation"
          titleComponent="h3"
          description="Set up the project dependencies"
        >
          <div className="mt-4 space-y-3">
            <div className="p-3 bg-muted rounded-md">
              <code className="text-sm">git clone https://github.com/example/repo.git</code>
            </div>
            <div className="p-3 bg-muted rounded-md">
              <code className="text-sm">cd repo && bun install</code>
            </div>
          </div>
        </Step>

        <Step
          icon={<FileText />}
          title="Configuration"
          description="Configure your environment settings"
        />

        <Step icon={<Rocket />} title="Launch" description="Start the development server" />
      </Steps>
    </div>
  ),
};

// Different title component types
export const TitleComponentTypes = {
  render: () => (
    <div className="space-y-12">
      <div className="max-w-md">
        <h3 className="text-sm font-medium mb-4">Span Elements (inline)</h3>
        <Steps titleComponent="span" size="sm">
          <Step title="Inline Step 1" description="Using span for inline titles" />
          <Step title="Inline Step 2" description="Good for compact layouts" />
          <Step title="Inline Step 3" description="No block-level spacing" />
        </Steps>
      </div>

      <div className="max-w-md">
        <h3 className="text-sm font-medium mb-4">Paragraph Elements</h3>
        <Steps titleComponent="p" currentStep={1}>
          <Step title="Paragraph Title" description="Using p tags for titles" />
          <Step title="Another Paragraph" description="Semantic when not a heading" />
          <Step title="Final Paragraph" description="Good for descriptive content" />
        </Steps>
      </div>

      <div className="max-w-md">
        <h3 className="text-sm font-medium mb-4">Div Elements (default)</h3>
        <Steps currentStep={2}>
          <Step title="Default Behavior" description="When no titleComponent is specified" />
          <Step title="Generic Container" description="Uses div as the default element" />
          <Step title="Flexible Styling" description="Most versatile option" />
        </Steps>
      </div>
    </div>
  ),
};
