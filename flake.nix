{
  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "nixpkgs/nixos-unstable";
    treefmt-nix = {
      url = "github:numtide/treefmt-nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs:
    inputs.flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import inputs.nixpkgs {
          localSystem = {
            inherit system;
          };
        };
        treefmt = inputs.treefmt-nix.lib.evalModule pkgs {
          projectRootFile = "flake.nix";
          programs.nixfmt.enable = true;
        };
        envtest-bins = pkgs.symlinkJoin {
          name = "envtest-bins";
          paths = [
            pkgs.etcd
            pkgs.kubernetes
          ];
        };
        deploy = pkgs.buildGoModule {
          pname = "deploy";
          version = "0.0.0";
          src = ./.;
          vendorHash = "sha256-afFOb7DpB4gGyFErwM3lROMU2E1GVlT7+nSLU4zAV8E=";

          subPackages = [
            "cmd/crypt"
            "cmd/deploy"
            "cmd/deployd"
            "cmd/hookd"
            "cmd/leakdetect"
          ];

          nativeBuildInputs = with pkgs; [
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc
          ];

          doCheck = false;
        };
      in
      {
        packages.default = deploy;

        devShells.default = pkgs.mkShell {
          inputsFrom = [
            deploy
            envtest-bins
          ];
          shellHook = ''
            export KUBEBUILDER_ASSETS="${envtest-bins}/bin"
            mkdir -p .testbin
            ln -sfn ${pkgs.etcd}/bin/etcd .testbin/etcd
            ln -sfn ${pkgs.kubernetes}/bin/kube-apiserver .testbin/kube-apiserver
            ln -sfn ${pkgs.kubernetes}/bin/kubectl .testbin/kubectl
          '';
        };

        formatter = treefmt.config.build.wrapper;
      }
    );
}
