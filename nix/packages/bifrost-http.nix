{
  pkgs,
  inputs,
  src,
  version,
  bifrost-ui,
}:
let
  lib = pkgs.lib;

  # Bifrost requires Go 1.27 (go.mod/go.work). Force Go 1.27 for buildGoModule.
  buildGoModule = pkgs.callPackage "${inputs.nixpkgs}/pkgs/build-support/go/module.nix" {
    go = pkgs.go_1_27 or pkgs.go;
  };

in
buildGoModule {
  pname = "bifrost-http";
  inherit version;
  inherit src;

  modRoot = "transports";
  subPackages = [ "bifrost-http" ];
  # transports builds against the core/, framework/ and plugins/ module
  # versions its go.mod pins, the same build the release image makes
  # (transports/Dockerfile: go mod download, GOWORK=off). The hash moves only
  # with transports/go.sum.
  vendorHash = lib.fakeHash;

  doCheck = false;

  env = {
    CGO_ENABLED = "1";
    GOWORK = "off";
  };

  nativeBuildInputs = with pkgs; [
    pkg-config
    gcc
  ];
  buildInputs = [ pkgs.sqlite ];

  preBuild = ''
    # Provide UI assets for //go:embed all:ui
    rm -rf bifrost-http/ui
    mkdir -p bifrost-http/ui
    if [ -d "${bifrost-ui}/ui" ]; then
      cp -R --no-preserve=mode,ownership,timestamps "${bifrost-ui}/ui/." bifrost-http/ui/
    else
      printf '%s\n' '<!doctype html><meta charset="utf-8"><title>Bifrost</title>' > bifrost-http/ui/index.html
    fi
  '';

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${version}"
  ];

  meta = {
    mainProgram = "bifrost-http";
    description = "Bifrost HTTP gateway";
    homepage = "https://github.com/maximhq/bifrost";
    license = lib.licenses.asl20;
  };
}