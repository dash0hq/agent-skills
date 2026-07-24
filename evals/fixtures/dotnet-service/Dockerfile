# Eval fixture: uninstrumented ASP.NET Core minimal API (see
# evals/fixtures/README.md). Deterministic build: pinned base images.
#
#   docker build -t dotnet-service-eval .
FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /src
COPY *.csproj ./
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /out --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:8.0
WORKDIR /app
COPY --from=build /out .
EXPOSE 8080
ENTRYPOINT ["dotnet", "dotnet-service.dll"]
