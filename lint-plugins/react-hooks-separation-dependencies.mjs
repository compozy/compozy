import {
  collectDeclaredNames,
  collectReferencedIdentifiers,
} from "./react-hooks-separation-ast.mjs";

export function isPascalCase(name) {
  return typeof name === "string" && /^[A-Z][a-zA-Z0-9]*$/.test(name);
}

export function isHookName(name) {
  return typeof name === "string" && /^use[A-Z]/.test(name);
}

export function hookNameToKebabFile(name) {
  return name.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase() + ".ts";
}

export function isValidHookLocation(filePath) {
  const normalizedPath = filePath.replace(/\\/g, "/");
  return (
    normalizedPath.includes("/hooks/") || /\/use-[a-z][a-z0-9-]*\.[tj]sx?$/.test(normalizedPath)
  );
}

function analyzeLocalDependencies(hookStatement, { importedNames, moduleDeclaredNames }) {
  const references = collectReferencedIdentifiers(hookStatement);
  const declaredInsideHook = collectDeclaredNames(hookStatement);

  const localDependencies = [];
  for (const identifier of references) {
    if (declaredInsideHook.has(identifier)) continue;
    if (importedNames.has(identifier)) continue;
    if (!moduleDeclaredNames.has(identifier)) continue;
    localDependencies.push(identifier);
  }

  return [...new Set(localDependencies)];
}

function resolveTypeDependencies({ initialTypeDependencies, importedNames, moduleDeclarationMap }) {
  const collectedTypeDependencies = new Set();
  const valueDependencies = new Set();
  const queue = [...initialTypeDependencies];

  while (queue.length > 0) {
    const currentTypeDependency = queue.shift();
    if (collectedTypeDependencies.has(currentTypeDependency)) continue;

    const declarationEntry = moduleDeclarationMap.get(currentTypeDependency);
    if (!declarationEntry || declarationEntry.declarationKind !== "type") continue;

    collectedTypeDependencies.add(currentTypeDependency);
    const declarationReferences = collectReferencedIdentifiers(declarationEntry.statement);
    const declarationNames = collectDeclaredNames(declarationEntry.statement);

    for (const referencedName of declarationReferences) {
      if (declarationNames.has(referencedName)) continue;
      if (importedNames.has(referencedName)) continue;

      const referencedDeclaration = moduleDeclarationMap.get(referencedName);
      if (!referencedDeclaration) continue;

      if (referencedDeclaration.declarationKind === "type") {
        if (!collectedTypeDependencies.has(referencedName)) {
          queue.push(referencedName);
        }
        continue;
      }

      valueDependencies.add(referencedName);
    }
  }

  return {
    typeDependencies: [...collectedTypeDependencies],
    valueDependencies: [...valueDependencies],
  };
}

export function collectEligibleHooks({
  hooks,
  importedNames,
  moduleDeclaredNames,
  moduleDeclarationMap,
  filePath,
  warnings,
}) {
  const allHookNames = new Set(hooks.map(hook => hook.name));

  const hookAnalysis = hooks.map(hook => {
    if (!hook.canExtract) {
      warnings.push(
        `[skip] ${filePath}: hook "${hook.name}" is declared in a multi-declarator variable statement`
      );
      return {
        hook,
        localDependencies: [],
        hookDependencies: [],
        typeDependencies: [],
        valueDependencies: [],
        blocked: true,
      };
    }

    const localDependencies = analyzeLocalDependencies(hook.statement, {
      importedNames,
      moduleDeclaredNames,
    });

    const hookDependencies = [];
    const initialTypeDependencies = [];
    const valueDependencies = [];

    for (const dependencyName of localDependencies) {
      if (allHookNames.has(dependencyName)) {
        hookDependencies.push(dependencyName);
        continue;
      }

      const declarationEntry = moduleDeclarationMap.get(dependencyName);
      if (declarationEntry?.declarationKind === "type") {
        initialTypeDependencies.push(dependencyName);
        continue;
      }

      valueDependencies.push(dependencyName);
    }

    const { typeDependencies, valueDependencies: typeValueDependencies } = resolveTypeDependencies({
      initialTypeDependencies,
      importedNames,
      moduleDeclarationMap,
    });
    valueDependencies.push(...typeValueDependencies);

    return {
      hook,
      localDependencies,
      hookDependencies,
      typeDependencies,
      valueDependencies: [...new Set(valueDependencies)],
      blocked: false,
    };
  });

  let eligible = hookAnalysis.filter(analysis => !analysis.blocked);
  while (true) {
    const eligibleNames = new Set(eligible.map(analysis => analysis.hook.name));
    const next = eligible.filter(analysis =>
      analysis.hookDependencies.every(dependencyName => eligibleNames.has(dependencyName))
    );
    if (next.length === eligible.length) break;
    eligible = next;
  }

  const eligibleNames = new Set(eligible.map(analysis => analysis.hook.name));
  for (const analysis of hookAnalysis) {
    if (analysis.blocked) {
      continue;
    }

    const blockedDependencies = analysis.hookDependencies.filter(
      dependencyName => !eligibleNames.has(dependencyName)
    );
    if (blockedDependencies.length > 0) {
      warnings.push(
        `[skip] ${filePath}: hook "${analysis.hook.name}" depends on local symbols (${blockedDependencies.join(
          ", "
        )}) that cannot be auto-moved safely`
      );
    }
  }

  return eligible;
}
